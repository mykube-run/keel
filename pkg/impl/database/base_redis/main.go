package base_redis

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/redis/go-redis/v9"
	"net"
	"net/url"
	"time"
)

var (
	TaskTTL    = 3600 * 6 // 6 hours
	BucketSize = time.Minute * 5
)

type Redis struct {
	s redis.Scripter
}

func NewCluster(dsn string) (*Redis, error) {
	opt, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}
	c := redis.NewClusterClient(opt.Cluster())
	if err = c.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("error pinging redis cluster: %w", err)
	}
	r := &Redis{s: c}
	if err = r.LoadScripts(); err != nil {
		return nil, fmt.Errorf("error loading scripts: %w", err)
	}
	return r, nil
}

func NewSentinel(dsn string) (*Redis, error) {
	opt, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}
	c := redis.NewFailoverClient(opt.Failover())
	if err = c.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("error pinging redis sentinel: %w", err)
	}
	r := &Redis{s: c}
	if err = r.LoadScripts(); err != nil {
		return nil, fmt.Errorf("error loading scripts: %w", err)
	}
	return r, nil
}

func NewSimple(dsn string) (*Redis, error) {
	opt, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}
	c := redis.NewClient(opt.Simple())
	if err = c.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("error pinging redis server: %w", err)
	}
	r := &Redis{s: c}
	if err = r.LoadScripts(); err != nil {
		return nil, fmt.Errorf("error loading scripts: %w", err)
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
	uids, err := json.Marshal(opt.Uids)
	if err != nil {
		return err
	}
	return updateTaskStatus.Run(ctx, r.s, []string{opt.TenantId}, string(uids), opt.Status).Err()
}

func parseDSN(dsn string) (*redis.UniversalOptions, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("error parsing redis dsn url: %w", err)
	}

	if u.Scheme != "redis" {
		return nil, fmt.Errorf("invalid redis dsn url scheme: %s", u.Scheme)
	}

	if u.Opaque != "" {
		return nil, fmt.Errorf("invalid redis dsn url, url is opaque: %s", dsn)
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

	return opt, nil
}

func getBucketTimestamp(t time.Time) int64 {
	tmp := t.Round(BucketSize)
	if tmp.After(t) {
		tmp = tmp.Add(-BucketSize)
	}
	return tmp.Unix()
}

type extendedTask struct {
	entity.Task
	TaskTimestamp   int64 `json:"__task_ts__"`
	BucketTimestamp int64 `json:"__bucket_ts__"`
}
