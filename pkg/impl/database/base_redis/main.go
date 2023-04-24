package base_redis

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/redis/go-redis/v9"
	"io"
	"net"
	"net/url"
	"strings"
	"time"
)

var (
	TaskTTL    = 3600 * 6 // 6 hours
	BucketSize = time.Minute * 5
)

type Redis struct {
	s redis.Scripter
	c io.Closer
}

func New(dsn string) (*Redis, error) {
	opt, typ, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}

	r := new(Redis)

	switch strings.ToLower(typ) {
	case "simple":
		c := redis.NewClient(opt.Simple())
		if err = c.Ping(context.Background()).Err(); err != nil {
			return nil, fmt.Errorf("error pinging redis server: %w", err)
		}
		r.s = c
		r.c = c
	case "sentinel":
		c := redis.NewFailoverClient(opt.Failover())
		if err = c.Ping(context.Background()).Err(); err != nil {
			return nil, fmt.Errorf("error pinging redis sentinel: %w", err)
		}
		r.s = c
		r.c = c
	case "cluster":
		c := redis.NewClusterClient(opt.Cluster())
		if err = c.Ping(context.Background()).Err(); err != nil {
			return nil, fmt.Errorf("error pinging redis cluster: %w", err)
		}
		r.s = c
		r.c = c
	default:
		return nil, fmt.Errorf("unsupported redis connection type: %v (available options are: simple, sentinel and cluster)", typ)
	}
	return r, nil
}

func (r *Redis) LoadScripts() error {
	ctx := context.Background()
	if err := updateTaskStatus.Load(ctx, r.s).Err(); err != nil {
		return err
	}
	if err := findPendingTasks.Load(ctx, r.s).Err(); err != nil {
		return err
	}
	if err := createTask.Load(ctx, r.s).Err(); err != nil {
		return err
	}
	if err := getTask.Load(ctx, r.s).Err(); err != nil {
		return err
	}
	if err := getTaskStatus.Load(ctx, r.s).Err(); err != nil {
		return err
	}
	return nil
}

func (r *Redis) CountTenantPendingTasks(ctx context.Context, opt types.CountTenantPendingTasksOption) (int64, error) {
	return countTenantPendingTasks.Run(ctx, r.s, []string{opt.TenantId}).Int64()
}

func (r *Redis) GetTask(ctx context.Context, opt types.GetTaskOption) (*entity.Task, error) {
	str, err := getTask.Run(ctx, r.s, []string{opt.TenantId}, opt.Uid).Text()
	if err != nil {
		return nil, err
	}
	t := new(entity.Task)
	if err = json.Unmarshal([]byte(str), t); err != nil {
		return nil, err
	}
	return t, nil
}

func (r *Redis) GetTaskStatus(ctx context.Context, opt types.GetTaskStatusOption) (enum.TaskStatus, error) {
	str, err := getTaskStatus.Run(ctx, r.s, []string{opt.TenantId}, opt.Uid).Text()
	return enum.TaskStatus(str), err
}

func (r *Redis) CreateTask(ctx context.Context, t entity.Task) error {
	et := extendedTask{
		Task:            t,
		TaskTimestamp:   t.CreatedAt.Unix(),
		BucketTimestamp: getBucketTimestamp(t.CreatedAt),
	}
	byt, err := json.Marshal(et)
	if err != nil {
		return err
	}
	return createTask.Run(ctx, r.s, []string{t.TenantId}, t.Uid, string(byt), TaskTTL).Err()
}

func (r *Redis) FindPendingTasks(ctx context.Context, opt types.FindPendingTasksOption) (entity.Tasks, error) {
	if opt.TenantId == nil {
		return nil, fmt.Errorf("tenantId cannot be nil")
	}
	strs, err := findPendingTasks.Run(ctx, r.s, []string{*opt.TenantId}, 500).StringSlice()
	if err != nil {
		return nil, err
	}
	tasks := make(entity.Tasks, 0)
	for _, v := range strs {
		t := new(entity.Task)
		if err = json.Unmarshal([]byte(v), t); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (r *Redis) UpdateTaskStatus(ctx context.Context, opt types.UpdateTaskStatusOption) error {
	if len(opt.Uids) == 0 {
		return nil
	}
	uids, err := json.Marshal(opt.Uids)
	if err != nil {
		return err
	}
	return updateTaskStatus.Run(ctx, r.s, []string{opt.TenantId}, string(uids), string(opt.Status)).Err()
}

func (r *Redis) Close() error {
	return r.c.Close()
}

// parseDSN parses redis datasource name. Code converted from https://github.com/gomodule/redigo/blob/master/redis/conn.go#L329
func parseDSN(dsn string) (*redis.UniversalOptions, string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, "", fmt.Errorf("error parsing redis dsn url: %w", err)
	}

	if u.Scheme != "redis" {
		return nil, "", fmt.Errorf("invalid redis dsn url scheme: %s", u.Scheme)
	}

	if u.Opaque != "" {
		return nil, "", fmt.Errorf("invalid redis dsn url, url is opaque: %s", dsn)
	}

	opt := new(redis.UniversalOptions)

	// As per the IANA draft spec, the host defaults to localhost and
	// the port defaults to 6379.
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		// assume port is missing
		host = u.Host
		port = "6379"
	}
	if host == "" {
		host = "localhost"
	}
	opt.Addrs = []string{net.JoinHostPort(host, port)}

	if u.User != nil {
		password, isSet := u.User.Password()
		username := u.User.Username()
		if isSet {
			if username != "" {
				// ACL
				opt.Username = username
				opt.Password = password
			} else {
				opt.Password = password
				// requirepass - user-info username:password with blank username
			}
		} else if username != "" {
			// requirepass - redis-cli compatibility which treats as single arg in user-info as a password
			opt.Password = username
		}
	}

	typ := u.Query().Get("type")
	if typ == "" {
		typ = "simple"
	}
	return opt, typ, nil
}

// extendedTask extends entity.Task with task & bucket timestamp, which are used to determine task bucket in Redis
type extendedTask struct {
	entity.Task

	// Task create timestamp
	TaskTimestamp int64 `json:"__task_ts__"`

	// Bucket here is a time period that contains tasks created within that time period.
	// A bucket is one of the buckets that we split 24 hours into several time periods evenly by BucketSize
	// starting from zero time of the day. e.g. when BucketSize default to 5m, we will get 288 buckets.
	// Bucket timestamp is the start point of the bucket, its tasks are created in `[BucketTimestamp, BucketTimestamp+BucketSize)`
	BucketTimestamp int64 `json:"__bucket_ts__"`
}

// getBucketTimestamp returns a bucket timestamp from give task create time & specified bucket size.
func getBucketTimestamp(t time.Time) int64 {
	tmp := t.Round(BucketSize)
	if tmp.After(t) {
		tmp = tmp.Add(-BucketSize)
	}
	return tmp.Unix()
}
