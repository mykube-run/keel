package types

import (
	"encoding/json"
	"github.com/mykube-run/keel/pkg/enum"
	"time"
)

// TaskMessage defines the message that a worker sends to scheduler
type TaskMessage struct {
	Type        enum.TaskMessageType `json:"type"`        // Task message type
	SchedulerId string               `json:"schedulerId"` // Scheduler id that the message is sent to
	WorkerId    string               `json:"workerId"`    // Worker id that the message is sent from
	Task        Task                 `json:"task"`        // Task details
	Timestamp   time.Time            `json:"timestamp"`   // Timestamp the message is sent
	Value       json.RawMessage      `json:"value"`       // Extra message value in JSON
}
