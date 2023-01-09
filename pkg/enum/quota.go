package enum

type ResourceType string

const (
	ResourceTypeCPU         ResourceType = "CPU"
	ResourceTypeMemory      ResourceType = "Memory"
	ResourceTypeStorage     ResourceType = "Storage"
	ResourceTypeGPU         ResourceType = "GPU"
	ResourceTypeConcurrency ResourceType = "Concurrency"
	ResourceTypeCustom      ResourceType = "Custom"
	ResourceTypePeak        ResourceType = "Peak"
)
