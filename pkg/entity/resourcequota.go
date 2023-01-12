package entity

// ResourceQuota tenant resource quota options. Resource quotas are optional, 0 means there is no limit for specified resource type
type ResourceQuota struct {
	TenantId string `json:"tenantId" bson:"tenantId"`

	Concurrency int64 `json:"concurrency" bson:"concurrency"` // Task concurrency quota
	CPU         int64 `json:"cpu" bson:"cpu"`                 // CPU quota in cores
	Custom      int64 `json:"custom" bson:"custom"`           // Custom resource quota
	GPU         int64 `json:"gpu" bson:"gpu"`                 // GPU quota in cores
	Memory      int64 `json:"memory" bson:"memory"`           // Memory quota in MB
	Storage     int64 `json:"storage" bson:"storage"`         // Storage quota in MB

	Peak float32 `json:"peak" bson:"peak"` // Actual or realtime resource usage can exceed quota in percent. Ranged from 0 to 1.0 (or higher)
}

func (q *ResourceQuota) Fields() []interface{} {
	return []interface{}{
		&q.TenantId, &q.CPU, &q.Memory, &q.Storage, &q.GPU, &q.Concurrency, &q.Custom, &q.Peak,
	}
}

func (q *ResourceQuota) ConcurrencyEnabled() bool {
	return q.Concurrency > 0
}

func (q *ResourceQuota) CPUEnabled() bool {
	return q.CPU > 0
}

func (q *ResourceQuota) CustomEnabled() bool {
	return q.Custom > 0
}

func (q *ResourceQuota) GPUEnabled() bool {
	return q.GPU > 0
}

func (q *ResourceQuota) MemoryEnabled() bool {
	return q.Memory > 0
}

func (q *ResourceQuota) StorageEnabled() bool {
	return q.Storage > 0
}

func (q *ResourceQuota) PeakEnabled() bool {
	return q.Peak > 0
}
