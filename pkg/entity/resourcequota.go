package entity

import (
	"database/sql"
	"github.com/mykube-run/keel/pkg/enum"
)

type ResourceQuota struct {
	Id          string
	TenantId    string
	Type        string
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

func NewResourceQuota(typ string, val int64) ResourceQuota {
	q := ResourceQuota{Type: typ}
	switch typ {
	case string(enum.ResourceTypeCPU):
		q.CPU = sql.NullInt64{
			Int64: val,
			Valid: true,
		}
	case string(enum.ResourceTypeMemory):
		q.Memory = sql.NullInt64{
			Int64: val,
			Valid: true,
		}
	case string(enum.ResourceTypeStorage):
		q.Storage = sql.NullInt64{
			Int64: val,
			Valid: true,
		}
	case string(enum.ResourceTypeGPU):
		q.GPU = sql.NullInt64{
			Int64: val,
			Valid: true,
		}
	case string(enum.ResourceTypeConcurrency):
		q.Concurrency = sql.NullInt64{
			Int64: val,
			Valid: true,
		}
	case string(enum.ResourceTypeCustom):
		q.Custom = sql.NullInt64{
			Int64: val,
			Valid: true,
		}
	case string(enum.ResourceTypePeak):
		q.Peak = sql.NullFloat64{
			Float64: float64(val),
			Valid:   true,
		}
	}
	return q
}
