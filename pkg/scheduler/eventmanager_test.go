package scheduler

import (
	"encoding/json"
	"github.com/mykube-run/keel/pkg/impl/logging"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-xorm/xorm"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"github.com/stretchr/testify/suite"
)

func TestEventManager(t *testing.T) {
	suite.Run(t, new(EventManagerSuite))
}

// EventManagerSuite is a test suite for EventManager
type EventManagerSuite struct {
	suite.Suite
	tmpDir      string
	dbPath      string
	eventMgr    *EventManager
	snapshotKey string
}

func (s *EventManagerSuite) SetupSuite() {
	var err error
	s.tmpDir, err = os.MkdirTemp("", "eventmgr-test")
	s.Require().NoError(err)
	s.dbPath = filepath.Join(s.tmpDir, "test-events.db")
	DefaultDBPath = s.dbPath
}

func (s *EventManagerSuite) TearDownSuite() {
	_ = os.RemoveAll(s.tmpDir)
}

func (s *EventManagerSuite) SetupTest() {
	// Clean up previous test database
	if _, err := os.Stat(s.dbPath); err == nil {
		_ = os.Remove(s.dbPath)
	}

	// Configure snapshot settings
	conf := config.DefaultFromEnv()

	// Create event manager
	var err error
	s.eventMgr, err = NewEventManager(conf.Snapshot, "test-scheduler", logging.NewDefaultLogger(nil))
	s.Require().NoError(err)

	s.snapshotKey = s.eventMgr.snapshotKey(0)
}

func (s *EventManagerSuite) TearDownTest() {
	if s.eventMgr != nil {
		_ = s.eventMgr.Close()
	}
}

func (s *EventManagerSuite) TestNewEventFromMessage() {
	// Test with task message containing value
	timestamp := time.Now()
	taskMsg := &types.TaskMessage{
		Type:      enum.ReportTaskStatus,
		WorkerId:  "worker-1",
		Timestamp: timestamp,
		Task: types.Task{
			TenantId: "tenant-1",
			Uid:      "task-1",
		},
		Value: json.RawMessage(`{"progress": 50, "error": "some error", "resourceUsage": {"cpu": 100, "memory": 128}}`),
	}

	event := NewEventFromMessage(taskMsg)
	s.Require().NotNil(event)
	s.Equal(string(enum.ReportTaskStatus), event.EventType)
	s.Equal("worker-1", event.WorkerId)
	s.Equal("tenant-1", event.TenantId)
	s.Equal("task-1", event.TaskId)
	s.True(timestamp.Sub(event.Timestamp) < time.Second)
	s.Equal(50, event.Progress)
	s.Equal("some error", event.Error)
	s.Equal(types.ResourceUsage{CPU: 100, Memory: 128}, event.ResourceUsage)

	// Test with task message without value
	taskMsg.Value = nil
	event = NewEventFromMessage(taskMsg)
	s.Require().NotNil(event)
	s.Equal(0, event.Progress)
	s.Empty(event.Error)
	s.Empty(event.ResourceUsage)
}

func (s *EventManagerSuite) TestNewEventFromTask() {
	task := &entity.Task{
		TenantId: "tenant-1",
		Uid:      "task-1",
	}

	event := NewEventFromTask(enum.TaskDispatched, task)
	s.Require().NotNil(event)
	s.Equal(string(enum.TaskDispatched), event.EventType)
	s.Empty(event.WorkerId)
	s.Equal("tenant-1", event.TenantId)
	s.Equal("task-1", event.TaskId)
	s.True(time.Now().Sub(event.Timestamp) < time.Second)
}

func (s *EventManagerSuite) TestInsertEvent() {
	event := &TaskEvent{
		EventType: string(enum.TaskDispatched),
		WorkerId:  "worker-1",
		TenantId:  "tenant-1",
		TaskId:    "task-1",
		Timestamp: time.Now(),
	}

	err := s.eventMgr.Insert(event)
	s.Require().NoError(err)

	// Verify event was inserted
	latest, err := s.eventMgr.Latest("tenant-1", "task-1")
	s.Require().NoError(err)
	s.Equal(event.EventType, latest.EventType)
	s.Equal(event.WorkerId, latest.WorkerId)
	s.Equal(event.TenantId, latest.TenantId)
	s.Equal(event.TaskId, latest.TaskId)
	s.True(event.Timestamp.Sub(latest.Timestamp) < time.Second)
}

func (s *EventManagerSuite) TestInsertEventWithMilestone() {
	// Test TaskDispatched milestone
	event := NewEventFromTask(enum.TaskDispatched, &entity.Task{TenantId: "tenant-1", Uid: "task-1"})
	err := s.eventMgr.Insert(event)
	s.Require().NoError(err)

	latest, err := s.eventMgr.Latest("tenant-1", "task-1")
	s.Require().NoError(err)
	s.Equal(MilestoneDispatched, latest.Milestone)

	// Test TaskStarted milestone
	event = NewEventFromTask(enum.TaskStarted, &entity.Task{TenantId: "tenant-1", Uid: "task-1"})
	err = s.eventMgr.Insert(event)
	s.Require().NoError(err)

	latest, err = s.eventMgr.Latest("tenant-1", "task-1")
	s.Require().NoError(err)
	s.Equal(MilestoneStarted, latest.Milestone)

	// Test ReportTaskStatus milestone
	event = NewEventFromTask(enum.ReportTaskStatus, &entity.Task{TenantId: "tenant-1", Uid: "task-1"})
	err = s.eventMgr.Insert(event)
	s.Require().NoError(err)

	latest, err = s.eventMgr.Latest("tenant-1", "task-1")
	s.Require().NoError(err)
	s.Equal(MilestoneLatest, latest.Milestone)
}

func (s *EventManagerSuite) TestIterateEvents() {
	// Insert multiple events
	now := time.Now()
	event1 := &TaskEvent{
		EventType: string(enum.TaskDispatched),
		TenantId:  "tenant-1",
		TaskId:    "task-1",
		Timestamp: now.Add(-3 * time.Second),
	}
	event2 := &TaskEvent{
		EventType: string(enum.TaskStarted),
		TenantId:  "tenant-1",
		TaskId:    "task-1",
		Timestamp: now.Add(-2 * time.Second),
	}
	event3 := &TaskEvent{
		EventType: string(enum.ReportTaskStatus),
		TenantId:  "tenant-1",
		TaskId:    "task-1",
		Timestamp: now.Add(-1 * time.Second),
	}

	_ = s.eventMgr.Insert(event1)
	_ = s.eventMgr.Insert(event2)
	_ = s.eventMgr.Insert(event3)

	// Iterate events and collect timestamps
	var timestamps []time.Time
	err := s.eventMgr.Iterate("tenant-1", "task-1", func(e *TaskEvent) bool {
		timestamps = append(timestamps, e.Timestamp)
		return true
	})
	s.Require().NoError(err)
	s.Len(timestamps, 3)
	s.True(timestamps[0].Before(timestamps[1]))
	s.True(timestamps[1].Before(timestamps[2]))

	// Test early termination
	var count int
	err = s.eventMgr.Iterate("tenant-1", "task-1", func(e *TaskEvent) bool {
		count++
		return count < 2
	})
	s.Require().NoError(err)
	s.Equal(2, count)
}

func (s *EventManagerSuite) TestTasks() {
	// Insert events for multiple tasks
	_ = s.eventMgr.Insert(&TaskEvent{TenantId: "tenant-1", TaskId: "task-1"})
	_ = s.eventMgr.Insert(&TaskEvent{TenantId: "tenant-1", TaskId: "task-2"})
	_ = s.eventMgr.Insert(&TaskEvent{TenantId: "tenant-2", TaskId: "task-1"})

	// Get tasks for tenant-1
	tasks, err := s.eventMgr.Tasks("tenant-1")
	s.Require().NoError(err)
	s.Len(tasks, 2)
	s.Contains(tasks, "task-1")
	s.Contains(tasks, "task-2")

	// Get tasks for non-existent tenant
	tasks, err = s.eventMgr.Tasks("non-existent")
	s.Require().NoError(err)
	s.Empty(tasks)
}

func (s *EventManagerSuite) TestCountRunningTasks() {
	// Insert events for multiple tasks
	_ = s.eventMgr.Insert(&TaskEvent{TenantId: "tenant-1", TaskId: "task-1"})
	_ = s.eventMgr.Insert(&TaskEvent{TenantId: "tenant-1", TaskId: "task-2"})
	_ = s.eventMgr.Insert(&TaskEvent{TenantId: "tenant-2", TaskId: "task-1"})

	// Count running tasks for tenant-1
	count, err := s.eventMgr.CountRunningTasks("tenant-1")
	s.Require().NoError(err)
	s.Equal(2, count)

	// Count running tasks for non-existent tenant
	count, err = s.eventMgr.CountRunningTasks("non-existent")
	s.Require().NoError(err)
	s.Equal(0, count)
}

func (s *EventManagerSuite) TestWorkerLoads() {
	// Insert events for different workers
	now := time.Now()
	_ = s.eventMgr.Insert(&TaskEvent{WorkerId: "worker-1", Timestamp: now})
	_ = s.eventMgr.Insert(&TaskEvent{WorkerId: "worker-1", Timestamp: now.Add(-30 * time.Minute)})
	_ = s.eventMgr.Insert(&TaskEvent{WorkerId: "worker-2", Timestamp: now.Add(-10 * time.Minute)})

	// Get worker loads
	loads, err := s.eventMgr.WorkerLoads()
	s.Require().NoError(err)
	s.Len(loads, 2)
	s.Equal(1, loads["worker-1"].Tasks) // Only recent task counts
	s.Equal(1, loads["worker-2"].Tasks)
}

func (s *EventManagerSuite) TestLatestEvent() {
	// Insert multiple events with different timestamps
	now := time.Now()
	oldEvent := &TaskEvent{
		TenantId:  "tenant-1",
		TaskId:    "task-1",
		Timestamp: now.Add(-5 * time.Minute),
	}
	newEvent := &TaskEvent{
		TenantId:  "tenant-1",
		TaskId:    "task-1",
		Timestamp: now,
	}

	_ = s.eventMgr.Insert(oldEvent)
	_ = s.eventMgr.Insert(newEvent)

	// Get latest event
	latest, err := s.eventMgr.Latest("tenant-1", "task-1")
	s.Require().NoError(err)
	s.Equal(newEvent.Timestamp, latest.Timestamp)
	s.Equal(newEvent.EventType, latest.EventType)

	// Test for non-existent task
	latest, err = s.eventMgr.Latest("tenant-1", "non-existent")
	s.Require().Error(err)
	s.Nil(latest)
	s.True(strings.Contains(err.Error(), "event not found"))
}

func (s *EventManagerSuite) TestDeleteEvents() {
	// Insert events
	_ = s.eventMgr.Insert(&TaskEvent{TenantId: "tenant-1", TaskId: "task-1"})
	_ = s.eventMgr.Insert(&TaskEvent{TenantId: "tenant-1", TaskId: "task-1"})
	_ = s.eventMgr.Insert(&TaskEvent{TenantId: "tenant-1", TaskId: "task-2"})

	// Delete events for task-1
	err := s.eventMgr.Delete("tenant-1", "task-1")
	s.Require().NoError(err)

	// Verify events are deleted
	latest, err := s.eventMgr.Latest("tenant-1", "task-1")
	s.Require().Error(err)
	s.Nil(latest)
	s.True(strings.Contains(err.Error(), "event not found"))

	// Verify task-2 events are still present
	latest, err = s.eventMgr.Latest("tenant-1", "task-2")
	s.Require().NoError(err)
	s.NotNil(latest)
}

func (s *EventManagerSuite) TestBackup() {
	// Perform backup
	_, err := s.eventMgr.Backup()
	s.Require().NoError(err)
}

func (s *EventManagerSuite) TestHotBackup() {
	// Create a temporary destination file
	destPath := filepath.Join(s.tmpDir, "backup.db")
	defer os.Remove(destPath)

	// Insert some data to the database
	_ = s.eventMgr.Insert(&TaskEvent{TenantId: "tenant-1", TaskId: "task-1"})

	// Perform hot backup
	err := s.eventMgr.hotBackup(destPath)
	s.Require().NoError(err)

	// Verify the backup file exists
	_, err = os.Stat(destPath)
	s.Require().NoError(err)

	// Open the backup database and check data
	destEngine, err := xorm.NewEngine("sqlite3", destPath)
	s.Require().NoError(err)
	defer destEngine.Close()

	var count int64
	count, err = destEngine.Count(&TaskEvent{TenantId: "tenant-1", TaskId: "task-1"})
	s.Require().NoError(err)
	s.Equal(int64(1), count)
}

func (s *EventManagerSuite) TestMaybeCompactEvents() {
	// Insert an old event and a new event
	oldTime := time.Now().Add(-5 * time.Minute)
	newTime := time.Now()

	oldEvent := &TaskEvent{
		TenantId:  "tenant-1",
		TaskId:    "task-1",
		EventType: string(enum.ReportTaskStatus),
		Timestamp: oldTime,
	}
	newEvent := &TaskEvent{
		TenantId:  "tenant-1",
		TaskId:    "task-1",
		EventType: string(enum.ReportTaskStatus),
		Timestamp: newTime,
	}

	session := s.eventMgr.engine.NewSession()
	defer session.Close()

	// Start transaction
	_ = session.Begin()

	// Insert both events
	_, _ = session.Insert(oldEvent)
	_, _ = session.Insert(newEvent)

	// Compact events
	err := s.eventMgr.maybeCompactEvents(session, newEvent)
	s.Require().NoError(err)

	// Commit transaction
	_ = session.Commit()

	// Verify old event was deleted
	var count int64
	count, err = s.eventMgr.engine.Where("tenant_id = ? AND task_id = ? AND timestamp < ?",
		"tenant-1", "task-1", newTime).Count(&TaskEvent{})
	s.Require().NoError(err)
	s.Equal(int64(0), count)
}
