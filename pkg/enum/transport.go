package enum

const (
	TransportTypeKafka = "kafka"
)

type TransportRole string

const (
	TransportRoleScheduler TransportRole = "Scheduler"
	TransportRoleWorker    TransportRole = "Worker"
)
