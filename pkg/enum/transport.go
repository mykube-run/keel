package enum

type TransportRole string

const (
	TransportRoleScheduler TransportRole = "Scheduler"
	TransportRoleWorker    TransportRole = "Worker"
)
