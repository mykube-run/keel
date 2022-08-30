package database

// Fields
const (
	TenantFields          = "t.id, t.uid, t.zone, t.priority, t.name, t.status, t.created_at, t.updated_at, t.last_active"
	TenantFieldsWithQuota = TenantFields +
		"q.id, q.tenant_id, q.type, q.cpu, q.memory, q.storage, q.gpu, q.concurrency, q.custom, q.peak"
)

// Statements
const (
	StmtActiveTenants          = `SELECT ` + TenantFields + ` FROM tenant t WHERE t.last_active >= ?`
	StmtActiveTenantsWithQuota = `SELECT ` + TenantFieldsWithQuota + ` FROM tenant t LEFT JOIN resourcequota q ON t.id = q.tenant_id WHERE t.last_active >= ?`
)
