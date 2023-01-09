package types

import (
	"github.com/mykube-run/keel/pkg/enum"
	"time"
)

type TenantResourceUsages map[int64]*TenantResourceUsage

type TenantResourceUsage struct {
	Active  map[int64]*TaskResourceUsage
	History []*TaskResourceUsage
	Stale   []*TaskResourceUsage
}

type TaskResourceUsage struct {
	TaskId   int64
	TaskType int
	TenantId int64
	Current  ResourceUsage
	History  []ResourceUsage
}

type ResourceUsage struct {
	Time        time.Time
	CPU         int64
	Memory      int64
	Storage     int64
	GPU         int64
	Concurrency int64
	Custom      int64
}

func (u *ResourceUsage) GetByType(typ string) int64 {
	if u == nil {
		return 0
	}
	switch typ {
	case string(enum.ResourceTypeCPU):
		return u.CPU
	case string(enum.ResourceTypeMemory):
		return u.Memory
	case string(enum.ResourceTypeStorage):
		return u.Storage
	case string(enum.ResourceTypeGPU):
		return u.GPU
	case string(enum.ResourceTypeConcurrency):
		return u.Concurrency
	case string(enum.ResourceTypeCustom):
		return u.Custom
	default:
		return 0
	}
}
