package mysql_redis

import (
	"context"
	"fmt"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/impl/database/base_redis"
	"github.com/mykube-run/keel/pkg/impl/database/mysql"
	"github.com/mykube-run/keel/pkg/types"
	"strings"
)

type MySQLRedis struct {
	mysql *mysql.MySQL
	redis *base_redis.Redis
}

func New(dsn string) (*MySQLRedis, error) {
	my, redis, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}
	m := new(MySQLRedis)
	m.mysql, err = mysql.New(my)
	if err != nil {
		return nil, err
	}
	m.redis, err = base_redis.New(redis)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *MySQLRedis) CreateTenant(ctx context.Context, t entity.Tenant) error {
	return m.mysql.CreateTenant(ctx, t)
}

func (m *MySQLRedis) ActivateTenants(ctx context.Context, opt types.ActivateTenantsOption) error {
	return m.mysql.ActivateTenants(ctx, opt)
}

func (m *MySQLRedis) GetTenant(ctx context.Context, opt types.GetTenantOption) (*entity.Tenant, error) {
	return m.mysql.GetTenant(ctx, opt)
}

func (m *MySQLRedis) FindActiveTenants(ctx context.Context, opt types.FindActiveTenantsOption) (entity.Tenants, error) {
	return m.mysql.FindActiveTenants(ctx, opt)
}

func (m *MySQLRedis) CountTenantPendingTasks(ctx context.Context, opt types.CountTenantPendingTasksOption) (int64, error) {
	return m.redis.CountTenantPendingTasks(ctx, opt)
}

func (m *MySQLRedis) GetTask(ctx context.Context, opt types.GetTaskOption) (*entity.Task, error) {
	return m.redis.GetTask(ctx, opt)
}

func (m *MySQLRedis) GetTaskStatus(ctx context.Context, opt types.GetTaskStatusOption) (enum.TaskStatus, error) {
	return m.redis.GetTaskStatus(ctx, opt)
}

func (m *MySQLRedis) FindPendingTasks(ctx context.Context, opt types.FindPendingTasksOption) (entity.Tasks, error) {
	return m.redis.FindPendingTasks(ctx, opt)
}

func (m *MySQLRedis) CreateTask(ctx context.Context, t entity.Task) error {
	return m.redis.CreateTask(ctx, t)
}

func (m *MySQLRedis) UpdateTaskStatus(ctx context.Context, opt types.UpdateTaskStatusOption) error {
	return m.redis.UpdateTaskStatus(ctx, opt)
}

func (m *MySQLRedis) Close() error {
	_ = m.mysql.Close()
	_ = m.redis.Close()
	return nil
}

// parseDSN parses dsn seperated by comma, returns mysql and redis dsn
func parseDSN(dsn string) (string, string, error) {
	dsn = strings.ReplaceAll(dsn, " ", "")

	// mysql://xxx,redis://xxx
	if strings.Contains(dsn, ",redis") {
		spl := strings.SplitN(dsn, ",redis", 2)
		if len(spl) != 2 {
			return "", "", fmt.Errorf("invalid mysql+redis dsn: %v", dsn)
		}
		return spl[0], spl[1], nil
	}

	// redis://xxx,mysql://xxx
	if strings.Contains(dsn, ",mysql") {
		spl := strings.SplitN(dsn, ",mysql", 2)
		if len(spl) != 2 {
			return "", "", fmt.Errorf("invalid mysql+redis dsn: %v", dsn)
		}
		return spl[1], spl[0], nil
	}

	return "", "", fmt.Errorf("invalid mysql+redis dsn: %v", dsn)
}
