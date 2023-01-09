package database

// Statements
const (
	// StmtInsertTenant and other statements for interacting with tenant table
	StmtInsertTenant            = `INSERT INTO tenant (uid, zone, priority, partition_name, name, status) VALUES (?, ?, ?, ?, ?, ?)`
	StmtGetTenant               = `SELECT t.uid, t.zone, t.priority, t.partition_name, t.name, t.status, t.created_at, t.updated_at, t.last_active, q.tenant_id, q.type, q.cpu, q.memory, q.storage, q.gpu, q.concurrency, q.custom, q.peak FROM tenant t LEFT JOIN resourcequota q ON t.uid = q.tenant_id WHERE t.uid = ?`
	StmtCountTenantPendingTasks = `SELECT COUNT(1) FROM usertask WHERE status = 'Pending' AND tenant_id = ? AND created_at BETWEEN ? AND ?`

	StmtInsertTask    = `INSERT INTO usertask (uid, tenant_id, handler, config, schedule_strategy, priority, progress, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	StmtGetTask       = `SELECT uid, tenant_id, handler, config, schedule_strategy, priority, progress, status, created_at, updated_at FROM usertask WHERE uid = ?`
	StmtGetTaskStatus = `SELECT status FROM usertask WHERE uid = ?`

	StmtInsertResourceQuota = `INSERT INTO resourcequota (tenant_id, type, cpu, memory, storage, gpu, concurrency, custom, peak) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
)

// Statement templates
// NOTE: uninject language or reference here to avoid error reminders in JetBrains IDE
const (
	// TemplateActivateTenants and other statement templates
	TemplateActivateTenants   = `UPDATE tenant SET last_active = ? WHERE uid IN (%v)`
	TemplateFindActiveTenants = `SELECT t.uid, t.zone, t.priority, t.partition_name, t.name, t.status, t.created_at, t.updated_at, t.last_active, q.tenant_id, q.type, q.cpu, q.memory, q.storage, q.gpu, q.concurrency, q.custom, q.peak FROM tenant t LEFT JOIN resourcequota q ON t.uid = q.tenant_id %v`

	TemplateFindRecentTasks  = `SELECT uid, tenant_id, handler, config, schedule_strategy, priority, progress, status, created_at, updated_at FROM usertask %v LIMIT 500`
	TemplateUpdateTaskStatus = `UPDATE usertask SET status = ? WHERE uid IN (%v)`
)
