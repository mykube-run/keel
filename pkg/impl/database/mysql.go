package database

import (
	"context"
	"database/sql"
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
	_, err := m.db.ExecContext(ctx, StmtInsertTenant, t.Uid, t.Zone, t.Priority, t.Partition, t.Name, t.Status)
	if err != nil {
		return err
	}

	_, err = m.db.ExecContext(ctx, StmtInsertResourceQuota, t.ResourceQuota.TenantId, t.ResourceQuota.Type,
		t.ResourceQuota.CPU, t.ResourceQuota.Memory, t.ResourceQuota.Storage, t.ResourceQuota.GPU,
		t.ResourceQuota.Concurrency, t.ResourceQuota.Custom, t.ResourceQuota.Peak)
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
	row := m.db.QueryRowContext(ctx, StmtGetTenant, opt.TenantId)
	t := new(entity.Tenant)
	err := row.Scan(t.FieldsWithQuota()...)
	if err != nil {
		return nil, err
	}
	return t, nil
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

func (m *MySQL) GetTask(ctx context.Context, opt types.GetTaskOption) (entity.Tasks, error) {
	tasks := entity.Tasks{}
	if opt.TaskType != enum.TaskTypeUserTask {
		return tasks, fmt.Errorf("unsupported task type: %v", opt.TaskType)
	}

	row := m.db.QueryRowContext(ctx, StmtGetTask, opt.Uid)
	t := new(entity.UserTask)
	err := row.Scan(t.Fields()...)
	if err != nil {
		return tasks, err
	}
	tasks.UserTasks = []*entity.UserTask{t}
	return tasks, nil
}

func (m *MySQL) GetTaskStatus(ctx context.Context, opt types.GetTaskStatusOption) (enum.TaskStatus, error) {
	if opt.TaskType != enum.TaskTypeUserTask {
		return "", fmt.Errorf("unsupported task type: %v", opt.TaskType)
	}
	row := m.db.QueryRowContext(ctx, StmtGetTaskStatus, opt.Uid)
	var s enum.TaskStatus
	err := row.Scan(&s)
	return s, err
}

func (m *MySQL) CreateTask(ctx context.Context, t entity.UserTask) error {
	_, err := m.db.ExecContext(ctx, StmtInsertTask, t.Uid, t.TenantId, t.Handler, t.Config, t.ScheduleStrategy, t.Priority, t.Progress, t.Status)
	return err
}

func (m *MySQL) FindRecentTasks(ctx context.Context, opt types.FindRecentTasksOption) (entity.Tasks, error) {
	tasks := entity.Tasks{}
	if opt.TaskType != enum.TaskTypeUserTask {
		return tasks, fmt.Errorf("unsupported task type: %v", opt.TaskType)
	}

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
	if opt.MinUserTaskId != nil {
		q = append(q, "uid >= ?")
		args = append(args, *opt.MinUserTaskId)
	}
	if len(q) > 0 {
		where = "WHERE " + strings.Join(q, " AND ")
	}
	stmt := fmt.Sprintf(TemplateFindRecentTasks, where)

	rows, err := m.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return tasks, noRowsAsNil(err)
	}
	for rows.Next() {
		t := new(entity.UserTask)
		if err = rows.Scan(t.Fields()...); err != nil {
			return tasks, err
		}
		tasks.UserTasks = append(tasks.UserTasks, t)
	}
	return tasks, nil
}

func (m *MySQL) UpdateTaskStatus(ctx context.Context, opt types.UpdateTaskStatusOption) error {
	if opt.TaskType != enum.TaskTypeUserTask {
		return fmt.Errorf("unsupported task type: %v", opt.TaskType)
	}
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
