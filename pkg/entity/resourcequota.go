package entity

import "database/sql"

type ResourceQuota struct {
	Id          string
	TenantId    string
	Type        int
	CPU         sql.NullInt64
	Memory      sql.NullInt64
	Storage     sql.NullInt64
	GPU         sql.NullInt64
	Concurrency sql.NullInt64
	Custom      sql.NullInt64
	Peak        sql.NullFloat64
}

func (q *ResourceQuota) Fields() []interface{} {
	return []interface{}{
		&q.Id, &q.TenantId, &q.Type, &q.CPU, &q.Memory, &q.Storage, &q.GPU, &q.Concurrency, &q.Custom, &q.Peak,
	}
}
