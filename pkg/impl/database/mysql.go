package database

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"time"
)

type MySQL struct {
	db *sql.DB
}

func NewMySQL(dsn string) (types.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err = db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping mysql: %w", err)
	}
	my := &MySQL{db: db}
	return my, nil
}

func (m *MySQL) CreateTenant(ctx context.Context, t entity.Tenant) error {
	return nil
}

func (m *MySQL) CreateTask(ctx context.Context, t entity.UserTask) error {
	return nil
}

func (m *MySQL) FindActiveTenants(ctx context.Context, opt types.FindActiveTenantsOption) (ts entity.Tenants, err error) {
	from := "1970-01-01T00:00:00Z"
	if opt.From != nil {
		from = opt.From.Format(enum.ISOFormat)
	}
	rows, err := m.db.QueryContext(ctx, StmtActiveTenantsWithQuota, from)
	if err != nil {
		return nil, noRows(err)
	}

	for rows.Next() {
		t := new(entity.Tenant)
		if err = rows.Scan(t.Fields()...); err != nil {
			return nil, err
		} else {
			ts = append(ts, t)
		}
	}
	return
}

func (m *MySQL) FindRecentTasks(ctx context.Context, opt types.FindRecentTasksOption) (entity.Tasks, error) {
	panic("implement me")
}

func (m *MySQL) GetTask(ctx context.Context, opt types.GetTaskOption) (entity.Tasks, error) {
	panic("implement me")
}

func (m *MySQL) UpdateTaskStatus(ctx context.Context, opt types.UpdateTaskStatusOption) error {
	panic("implement me")
}

func (m *MySQL) GetTaskStatus(ctx context.Context, opt types.GetTaskStatusOption) (enum.TaskStatus, error) {
	return "", nil
}

func (m *MySQL) GetTenant(ctx context.Context, opt types.GetTenantOption) (entity.Tenant, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MySQL) CountTenantPendingTasks(ctx context.Context, opt types.CountTenantPendingTasksOption) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MySQL) ActivateTenants(ctx context.Context, opt types.ActivateTenantsOption) error {
	//TODO implement me
	panic("implement me")
}

func (m *MySQL) Close() error {
	return m.db.Close()
}

func noRows(err error) error {
	if err == sql.ErrNoRows {
		return nil
	} else {
		return err
	}
}
