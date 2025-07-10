package mongodb

import (
	"context"
	"errors"
	"fmt"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type MongoDB struct {
	db *mongo.Database

	// Collections
	tenant *mongo.Collection
	quota  *mongo.Collection
	task   *mongo.Collection
}

// DatabaseName is the database name in MongoDB, modify this variable to change database
var DatabaseName = "keel"

func New(dsn string) (*MongoDB, error) {
	opt := new(options.ClientOptions).ApplyURI(dsn)
	client, err := mongo.NewClient(opt)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	if err = client.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect monogdb: %v", err)
	}
	if err = client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping mongodb: %w", err)
	}

	db := client.Database(DatabaseName)
	m := &MongoDB{
		db:     db,
		tenant: db.Collection("tenant"),
		quota:  db.Collection("resourcequota"),
		task:   db.Collection("task"),
	}
	m.createIndices()
	return m, nil
}

func (m *MongoDB) CreateTenant(ctx context.Context, t entity.Tenant) error {
	_, err := m.getTenant(ctx, types.GetTenantOption{TenantId: t.Uid})
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return enum.ErrTenantAlreadyExists
	}

	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = now
	}
	if t.LastActive.IsZero() {
		t.LastActive = now
	}
	_, err = m.tenant.InsertOne(ctx, t)
	if err != nil {
		return err
	}
	_, err = m.quota.InsertOne(ctx, t.ResourceQuota)
	return err
}

func (m *MongoDB) ActivateTenants(ctx context.Context, opt types.ActivateTenantsOption) error {
	if len(opt.TenantId) == 0 {
		return nil
	}

	q := bson.M{"uid": bson.M{"$in": opt.TenantId}}
	u := bson.D{{"$set", bson.D{{"lastActive", opt.ActiveTime}}}}
	_, err := m.tenant.UpdateMany(ctx, q, u)
	return err
}

func (m *MongoDB) GetTenant(ctx context.Context, opt types.GetTenantOption) (result *entity.Tenant, err error) {
	return m.getTenant(ctx, opt)
}

func (m *MongoDB) FindActiveTenants(ctx context.Context, opt types.FindActiveTenantsOption) (tenants entity.Tenants, err error) {
	q := bson.M{}

	if opt.From != nil {
		q["lastActive"] = bson.M{"$gte": *opt.From}
	}
	if opt.Zone != nil {
		q["zone"] = *opt.Zone
	}
	if opt.Partition != nil {
		q["partition"] = *opt.Partition
	}

	cur, err := m.tenant.Find(ctx, q)
	if err != nil {
		return nil, err
	}
	if err = cur.All(ctx, &tenants); err != nil {
		return nil, err
	}
	err = m.findTenantQuota(ctx, tenants)
	return tenants, err
}

func (m *MongoDB) CountTenantPendingTasks(ctx context.Context, opt types.CountTenantPendingTasksOption) (int64, error) {
	q := bson.M{
		"tenantId":  opt.TenantId,
		"createdAt": bson.M{"$gt": opt.From, "$lt": opt.To},
	}
	cnt, err := m.task.CountDocuments(ctx, q)
	return cnt, err
}

func (m *MongoDB) GetTask(ctx context.Context, opt types.GetTaskOption) (*entity.Task, error) {
	task := new(entity.Task)
	q := bson.M{"uid": opt.Uid}
	res := m.task.FindOne(ctx, q)
	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return nil, enum.ErrTaskNotFound
		}
		return nil, res.Err()
	}
	if err := res.Decode(task); err != nil {
		return nil, err
	}
	return task, nil
}

func (m *MongoDB) GetTaskStatus(ctx context.Context, opt types.GetTaskStatusOption) (enum.TaskStatus, error) {
	q := bson.M{"uid": opt.Uid, "tenantId": opt.TenantId}
	res := m.task.FindOne(ctx, q)
	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return "", enum.ErrTaskNotFound
		}
		return "", res.Err()
	}
	t := new(entity.Task)
	err := res.Decode(t)
	if err != nil {
		return "", err
	}
	return t.Status, nil
}

func (m *MongoDB) CreateTask(ctx context.Context, t entity.Task) error {
	_, err := m.task.InsertOne(ctx, t)
	return err
}

func (m *MongoDB) FindPendingTasks(ctx context.Context, opt types.FindPendingTasksOption) (entity.Tasks, error) {
	q := bson.M{}
	if len(opt.Status) > 0 {
		q["status"] = bson.M{"$in": opt.Status}
	}
	if opt.TenantId != nil {
		q["tenantId"] = *opt.TenantId
	}
	if opt.MinUid != nil {
		q["uid"] = bson.M{"$gte": *opt.MinUid}
	}
	q["createdAt"] = bson.M{"$gte": time.Now().Add(-1 * enum.PendingTaskExpiration)}

	tasks := make(entity.Tasks, 0)
	limit := int64(300)
	cur, err := m.task.Find(ctx, q, &options.FindOptions{Sort: bson.M{"createdAt": 1}, Limit: &limit})
	if err != nil {
		return nil, err
	}
	if err = cur.All(ctx, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (m *MongoDB) UpdateTaskStatus(ctx context.Context, opt types.UpdateTaskStatusOption) error {
	if len(opt.Uids) == 0 {
		return nil
	}
	q := bson.M{"uid": bson.M{"$in": opt.Uids}}
	u := bson.D{{"$set", bson.D{{"status", opt.Status}, {"updatedAt", time.Now()}}}}
	_, err := m.task.UpdateMany(ctx, q, u)
	return err
}

func (m *MongoDB) Close() error {
	return nil
}

func (m *MongoDB) createIndices() {
	ctx := context.Background()
	// Tenant indices
	// uid_1
	{
		keys := primitive.D{}
		keys = append(keys, primitive.E{Key: "uid", Value: 1})
		opt := new(options.IndexOptions).SetUnique(true)
		idx := mongo.IndexModel{
			Keys:    keys,
			Options: opt,
		}
		_, err := m.tenant.Indexes().CreateOne(ctx, idx)
		if err != nil {
			log.Err(err).Msg("error creating index")
		}
	}
	// zone_1_partition_1
	{
		keys := primitive.D{}
		keys = append(keys, primitive.E{Key: "zone", Value: 1})
		keys = append(keys, primitive.E{Key: "partition", Value: 1})
		idx := mongo.IndexModel{
			Keys: keys,
		}
		_, err := m.tenant.Indexes().CreateOne(ctx, idx)
		if err != nil {
			log.Err(err).Msg("error creating index")
		}
	}

	// task indices
	// uid_1
	{
		keys := primitive.D{}
		keys = append(keys, primitive.E{Key: "uid", Value: 1})
		opt := new(options.IndexOptions).SetUnique(true)
		idx := mongo.IndexModel{
			Keys:    keys,
			Options: opt,
		}
		_, err := m.task.Indexes().CreateOne(ctx, idx)
		if err != nil {
			log.Err(err).Msg("error creating index")
		}
	}
	// handler_1
	{
		keys := primitive.D{}
		keys = append(keys, primitive.E{Key: "handler", Value: 1})
		idx := mongo.IndexModel{
			Keys: keys,
		}
		_, err := m.task.Indexes().CreateOne(ctx, idx)
		if err != nil {
			log.Err(err).Msg("error creating index")
		}
	}
	// tenantId
	// tenantId_1_status_1
	{
		keys := primitive.D{}
		keys = append(keys, primitive.E{Key: "tenantId", Value: 1}, primitive.E{Key: "status", Value: 1})
		idx := mongo.IndexModel{
			Keys:    keys,
			Options: new(options.IndexOptions),
		}
		_, err := m.task.Indexes().CreateOne(ctx, idx)
		if err != nil {
			log.Err(err).Msg("error creating index")
		}
	}

	// resourcequota indices
	// tenantId_1
	{
		keys := primitive.D{}
		keys = append(keys, primitive.E{Key: "tenantId", Value: 1})
		opt := new(options.IndexOptions).SetUnique(true)
		idx := mongo.IndexModel{
			Keys:    keys,
			Options: opt,
		}
		_, err := m.quota.Indexes().CreateOne(ctx, idx)
		if err != nil {
			log.Err(err).Msg("error creating index")
		}
	}
}

func (m *MongoDB) getTenant(ctx context.Context, opt types.GetTenantOption) (t *entity.Tenant, err error) {
	q := bson.M{"uid": opt.TenantId}
	res := m.tenant.FindOne(ctx, q)
	if res.Err() != nil {
		return nil, res.Err()
	}
	t = new(entity.Tenant)
	if err = res.Decode(t); err != nil {
		return nil, err
	}

	q = bson.M{"tenantId": opt.TenantId}
	res = m.quota.FindOne(ctx, q)
	if res.Err() != nil {
		return nil, res.Err()
	}
	quota := new(entity.ResourceQuota)
	if err = res.Decode(quota); err != nil {
		return nil, err
	}
	t.ResourceQuota = *quota
	return
}

func (m *MongoDB) findTenantQuota(ctx context.Context, tenants entity.Tenants) error {
	if len(tenants) == 0 {
		return nil
	}

	ids := make([]string, 0)
	for _, t := range tenants {
		ids = append(ids, t.Uid)
	}
	q := bson.M{
		"tenantId": bson.M{"$in": ids},
	}
	cur, err := m.quota.Find(ctx, q)
	if err != nil {
		return err
	}
	qs := make([]entity.ResourceQuota, 0)
	if err = cur.All(ctx, &qs); err != nil {
		return err
	}
	qm := make(map[string]entity.ResourceQuota)
	for i, v := range qs {
		qm[v.TenantId] = qs[i]
	}
	for i, t := range tenants {
		tenants[i].ResourceQuota = qm[t.Uid]
	}
	return nil
}
