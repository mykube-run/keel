package mongodb_redis

import (
	"context"
	"fmt"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/impl/database/base_redis"
	"github.com/mykube-run/keel/pkg/impl/database/mongodb"
	"github.com/mykube-run/keel/pkg/types"
	"strings"
)

type MongoDBRedis struct {
	mongo *mongodb.MongoDB
	redis *base_redis.Redis
}

func New(dsn string) (*MongoDBRedis, error) {
	mongo, redis, err := parseDSN(dsn)
	if err != nil {
		return nil, err
	}
	m := new(MongoDBRedis)
	m.mongo, err = mongodb.New(mongo)
	if err != nil {
		return nil, err
	}
	m.redis, err = base_redis.New(redis)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *MongoDBRedis) CreateTenant(ctx context.Context, t entity.Tenant) error {
	return m.mongo.CreateTenant(ctx, t)
}

func (m *MongoDBRedis) ActivateTenants(ctx context.Context, opt types.ActivateTenantsOption) error {
	return m.mongo.ActivateTenants(ctx, opt)
}

func (m *MongoDBRedis) GetTenant(ctx context.Context, opt types.GetTenantOption) (*entity.Tenant, error) {
	return m.mongo.GetTenant(ctx, opt)
}

func (m *MongoDBRedis) FindActiveTenants(ctx context.Context, opt types.FindActiveTenantsOption) (entity.Tenants, error) {
	return m.mongo.FindActiveTenants(ctx, opt)
}

func (m *MongoDBRedis) CountTenantPendingTasks(ctx context.Context, opt types.CountTenantPendingTasksOption) (int64, error) {
	return m.redis.CountTenantPendingTasks(ctx, opt)
}

func (m *MongoDBRedis) GetTask(ctx context.Context, opt types.GetTaskOption) (*entity.Task, error) {
	return m.redis.GetTask(ctx, opt)
}

func (m *MongoDBRedis) GetTaskStatus(ctx context.Context, opt types.GetTaskStatusOption) (enum.TaskStatus, error) {
	return m.redis.GetTaskStatus(ctx, opt)
}

func (m *MongoDBRedis) FindPendingTasks(ctx context.Context, opt types.FindPendingTasksOption) (entity.Tasks, error) {
	return m.redis.FindPendingTasks(ctx, opt)
}

func (m *MongoDBRedis) CreateTask(ctx context.Context, t entity.Task) error {
	return m.redis.CreateTask(ctx, t)
}

func (m *MongoDBRedis) UpdateTaskStatus(ctx context.Context, opt types.UpdateTaskStatusOption) error {
	return m.redis.UpdateTaskStatus(ctx, opt)
}

func (m *MongoDBRedis) Close() error {
	_ = m.mongo.Close()
	_ = m.redis.Close()
	return nil
}

// parseDSN parses dsn seperated by comma, returns mongodb and redis dsn
func parseDSN(dsn string) (string, string, error) {
	dsn = strings.ReplaceAll(dsn, " ", "")

	// mongodb://xxx,redis://xxx
	if strings.Contains(dsn, ",redis") {
		spl := strings.SplitN(dsn, ",redis", 2)
		if len(spl) != 2 {
			return "", "", fmt.Errorf("invalid mongodb+redis dsn: %v", dsn)
		}
		return spl[0], spl[1], nil
	}

	// redis://xxx,mongodb://xxx
	if strings.Contains(dsn, ",mongodb") {
		spl := strings.SplitN(dsn, ",mongodb", 2)
		if len(spl) != 2 {
			return "", "", fmt.Errorf("invalid mongodb+redis dsn: %v", dsn)
		}
		return spl[1], spl[0], nil
	}

	return "", "", fmt.Errorf("invalid mongodb+redis dsn: %v", dsn)
}
