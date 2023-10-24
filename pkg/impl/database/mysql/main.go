package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"strings"
	"time"
)

type MySQL struct {
	db *sql.DB
}

func New(dsn string) (*MySQL, error) {
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
	_, err := m.getTenant(ctx, types.GetTenantOption{TenantId: t.Uid})
	if err != sql.ErrNoRows {
		return enum.ErrTenantAlreadyExists
	}
	_, err = m.db.ExecContext(ctx, StmtInsertTenant, t.Uid, t.Zone, t.Priority, t.Partition, t.Name, t.Status)
	if err != nil {
		return err
	}
	_, err = m.db.ExecContext(ctx, StmtInsertResourceQuota,
		t.ResourceQuota.TenantId, t.ResourceQuota.Concurrency, t.ResourceQuota.CPU, t.ResourceQuota.Custom,
		t.ResourceQuota.GPU, t.ResourceQuota.Memory, t.ResourceQuota.Storage, t.ResourceQuota.Peak)
	return err
}

func (m *MySQL) ActivateTenants(ctx context.Context, opt types.ActivateTenantsOption) error {
	if len(opt.TenantId) == 0 {
		return nil
	}
	args := []interface{}{opt.ActiveTime}
	for i, _ := range opt.TenantId {
		args = append(args, opt.TenantId[i])
	}
	stmt := fmt.Sprintf(TemplateActivateTenants, inPlaceHolders(len(opt.TenantId)))
	_, err := m.db.ExecContext(ctx, stmt, args...)
	return err
}

func (m *MySQL) GetTenant(ctx context.Context, opt types.GetTenantOption) (*entity.Tenant, error) {
	return m.getTenant(ctx, opt)
}

func (m *MySQL) FindActiveTenants(ctx context.Context, opt types.FindActiveTenantsOption) (ts entity.Tenants, err error) {
	q := make([]string, 0)
	args := make([]interface{}, 0)
	where := ""

	if opt.From != nil {
		q = append(q, "t.last_active >= ?")
		args = append(args, opt.From.Format(enum.ISOFormat))
	}
	if opt.Zone != nil {
		q = append(q, "t.zone = ?")
		args = append(args, *opt.Zone)
	}
	if opt.Partition != nil {
		q = append(q, "t.partition_name = ?")
		args = append(args, *opt.Partition)
	}

	if len(q) > 0 {
		where = "WHERE " + strings.Join(q, " AND ")
	}
	stmt := fmt.Sprintf(TemplateFindActiveTenants, where)
	rows, err := m.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, noRowsAsNil(err)
	}

	for rows.Next() {
		t := new(entity.Tenant)
		if err = rows.Scan(t.FieldsWithQuota()...); err != nil {
			return nil, err
		} else {
			ts = append(ts, t)
		}
	}
	return
}

func (m *MySQL) CountTenantPendingTasks(ctx context.Context, opt types.CountTenantPendingTasksOption) (int64, error) {
	row := m.db.QueryRowContext(ctx, StmtCountTenantPendingTasks, opt.TenantId, opt.From, opt.To)
	var n int64
	err := row.Scan(&n)
	return n, err
}

func (m *MySQL) GetTask(ctx context.Context, opt types.GetTaskOption) (*entity.Task, error) {
	row := m.db.QueryRowContext(ctx, StmtGetTask, opt.Uid)
	task := new(entity.Task)
	err := row.Scan(task.Fields()...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, enum.ErrTaskNotFound
		}
		return nil, err
	}
	return task, nil
}

func (m *MySQL) GetTaskStatus(ctx context.Context, opt types.GetTaskStatusOption) (enum.TaskStatus, error) {
	row := m.db.QueryRowContext(ctx, StmtGetTaskStatus, opt.Uid)
	var s enum.TaskStatus
	err := row.Scan(&s)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return "", enum.ErrTaskNotFound
	}
	return s, err
}

func (m *MySQL) CreateTask(ctx context.Context, t entity.Task) error {
	_, err := m.db.ExecContext(ctx, StmtInsertTask, t.Uid, t.TenantId, t.Handler, t.Config, t.ScheduleStrategy, t.Priority, t.Progress, t.Status)
	return err
}

func (m *MySQL) FindPendingTasks(ctx context.Context, opt types.FindPendingTasksOption) (entity.Tasks, error) {
	tasks := make(entity.Tasks, 0)
	q := make([]string, 0)
	args := make([]interface{}, 0)
	where := ""

	if len(opt.Status) > 0 {
		q = append(q, fmt.Sprintf("status IN (%v)", inPlaceHolders(len(opt.Status))))
		for i, _ := range opt.Status {
			args = append(args, opt.Status[i])
		}
	}
	if opt.TenantId != nil {
		q = append(q, "tenant_id = ?")
		args = append(args, *opt.TenantId)
	}
	if opt.MinUid != nil {
		q = append(q, "uid >= ?")
		args = append(args, *opt.MinUid)
	}
	if len(q) > 0 {
		where = "WHERE " + strings.Join(q, " AND ")
	}
	stmt := fmt.Sprintf(TemplateFindPendingTasks, where)

	rows, err := m.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, noRowsAsNil(err)
	}
	for rows.Next() {
		t := new(entity.Task)
		if err = rows.Scan(t.Fields()...); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (m *MySQL) UpdateTaskStatus(ctx context.Context, opt types.UpdateTaskStatusOption) error {
	if len(opt.Uids) == 0 {
		return nil
	}

	args := []interface{}{opt.Status}
	for i, _ := range opt.Uids {
		args = append(args, opt.Uids[i])
	}
	stmt := fmt.Sprintf(TemplateUpdateTaskStatus, inPlaceHolders(len(opt.Uids)))
	_, err := m.db.ExecContext(ctx, stmt, args...)
	return err
}

func (m *MySQL) Close() error {
	return m.db.Close()
}

func (m *MySQL) getTenant(ctx context.Context, opt types.GetTenantOption) (*entity.Tenant, error) {
	row := m.db.QueryRowContext(ctx, StmtGetTenant, opt.TenantId)
	t := new(entity.Tenant)
	err := row.Scan(t.FieldsWithQuota()...)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func noRowsAsNil(err error) error {
	if err == sql.ErrNoRows {
		return nil
	} else {
		return err
	}
}

// inPlaceHolders returns n question-mark (?) place holders seperated by ', '
func inPlaceHolders(n int) string {
	if n == 0 {
		return ""
	}

	tmp := make([]string, n)
	for i := 0; i < n; i++ {
		tmp[i] = "?"
	}
	return strings.Join(tmp, ", ")
}
