package types

import (
	"github.com/mykube-run/keel/pkg/enum"
)

//type TenantResourceUsages map[int64]*TenantResourceUsage
//
//type TenantResourceUsage struct {
//	Active  map[string]*TaskResourceUsage
//	History []*TaskResourceUsage
//	Stale   []*TaskResourceUsage
//}
//
//type TaskResourceUsage struct {
//	TaskId   string
//	TaskType string
//	TenantId string
//	Current  ResourceUsage
//	History  []ResourceUsage
//}

// WorkerLoads defines worker loads indexed by worker id
type WorkerLoads map[string]WorkerLoad

// WorkerLoad defines worker load
type WorkerLoad struct {
	Tasks int `json:"tasks"`
	// ResourceUsage ResourceUsage `json:"resourceUsage"`
}

// ResourceUsage defines task resource usage
type ResourceUsage struct {
	CPU         int `json:"cpu"`
	Memory      int `json:"memory"`
	Storage     int `json:"storage"`
	GPU         int `json:"gpu"`
	Concurrency int `json:"concurrency"`
	Custom      int `json:"custom"`
}

func (u *ResourceUsage) GetByType(typ string) int {
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
