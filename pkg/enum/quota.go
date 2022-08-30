package enum

const (
	ResourceTypeCPU = iota + 1
	ResourceTypeMemory
	ResourceTypeStorage
	ResourceTypeGPU
	ResourceTypeConcurrency
	ResourceTypeCustom
	// ResourceTypePeak
)
