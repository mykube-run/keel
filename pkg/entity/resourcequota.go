package entity

import (
	"database/sql"
	"github.com/mykube-run/keel/pkg/enum"
)

type ResourceQuota struct {
	Id          string          `json:"id" bson:"_id"`
	TenantId    string          `json:"tenantId" bson:"tenantId"`
	Type        string          `json:"type" bson:"type"`
	CPU         sql.NullInt64   `json:"cpu" bson:"cpu"`
	Memory      sql.NullInt64   `json:"memory" bson:"memory"`
	Storage     sql.NullInt64   `json:"storage" bson:"storage"`
	GPU         sql.NullInt64   `json:"gpu" bson:"gpu"`
	Concurrency sql.NullInt64   `json:"concurrency" bson:"concurrency"`
	Custom      sql.NullInt64   `json:"custom" bson:"custom"`
	Peak        sql.NullFloat64 `json:"peak" bson:"peak"`
}

func (q *ResourceQuota) Fields() []interface{} {
	return []interface{}{
		&q.TenantId, &q.Type, &q.CPU, &q.Memory, &q.Storage, &q.GPU, &q.Concurrency, &q.Custom, &q.Peak,
	}
}

func NewResourceQuota(tid, typ string, val int64) ResourceQuota {
	q := ResourceQuota{
		Type:     typ,
		TenantId: tid,
	}
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
