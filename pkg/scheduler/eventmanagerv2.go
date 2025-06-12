package scheduler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-xorm/xorm"
	_ "github.com/mattn/go-sqlite3"
	"github.com/minio/minio-go"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
)

// Configurable variables
var (
	DefaultDBPath           = "./events.db"
	DefaultDBSnapshotPrefix = "events-db-snapshot"
	// DefaultEventCompactDuration = 60 // 60 seconds
	// DefaultTaskCounterTTL       = 30 // 30 seconds
	// DefaultWALCheckpointInterval = 1000  // SQLite WAL Configuration, checkpoint interval (page)
	// DefaultWALAutoCheckpoint     = 60    // SQLite WAL Configuration, auto checkpoint (second)
	// DefaultSyncMode              = "NORMAL" // SQLite WAL Configuration, sync mode, OFF means do not sync
)

const (
	MilestoneDispatched = "dispatched"
	MilestoneStarted    = "started"
)

// TaskEvent is the task event stored in local database
type TaskEvent struct {
	ID        int64           `xorm:"pk autoincr 'id'"`
	EventType string          `xorm:"varchar(100) null 'event_type'"`
	Milestone string          `xorm:"varchar(100) null 'milestone'"`
	WorkerId  string          `xorm:"varchar(256) index(idx_worker_id) 'worker_id'"`
	TenantId  string          `xorm:"varchar(256) index(idx_tenant_task) 'tenant_id'"`
	TaskId    string          `xorm:"varchar(256) index(idx_tenant_task) 'task_id'"`
	Timestamp time.Time       `xorm:"timestamp null 'timestamp'"`
	Value     json.RawMessage `xorm:"text null 'value'"`
}

func (TaskEvent) TableName() string {
	return "task_events"
}

// NewEventFromMessage creates a new event from task message
func NewEventFromMessage(m *types.TaskMessage) *TaskEvent {
	return &TaskEvent{
		EventType: string(m.Type),
		WorkerId:  m.WorkerId,
		TenantId:  m.Task.TenantId,
		TaskId:    m.Task.Uid,
		Timestamp: m.Timestamp,
		Value:     m.Value,
	}
}

// NewEventFromTask creates a new event from event type and task entity
func NewEventFromTask(typ enum.TaskMessageType, t *entity.Task) *TaskEvent {
	return &TaskEvent{
		EventType: string(typ),
		WorkerId:  "",
		TenantId:  t.TenantId,
		TaskId:    t.Uid,
		Timestamp: time.Now(),
		Value:     nil,
	}
}

// EventManager is the event manager that tracks task events
type EventManager struct {
	engine  *xorm.Engine
	sc      config.SnapshotConfig
	s3      *minio.Client
	sv      int // snapshot version
	lg      types.Logger
	sched   string
	ticker  *time.Ticker
	stopped chan bool
}

// NewEventManager creates an event manager instance
func NewEventManager(sc config.SnapshotConfig, schedulerId string, lg types.Logger) (*EventManager, error) {
	m := &EventManager{
		sc:      sc,
		lg:      lg,
		sched:   schedulerId,
		stopped: make(chan bool),
	}

	// 1. Initialize S3 client and load snapshot from s3 if enabled
	if sc.Enabled {
		client, err := minio.NewV4(sc.Endpoint, sc.AccessKey, sc.AccessSecret, sc.Secure)
		if err != nil {
			return nil, fmt.Errorf("error initializing s3 client: %w", err)
		}
		m.s3 = client
		if err = m.loadSnapshot(); err != nil {
			return nil, fmt.Errorf("error loading snapshot from s3: %w", err)
		}
	}

	// 2. Open the local database
	// connStr := fmt.Sprintf("% s?_journal=WAL&_synchronous=% s&_wal_autocheckpoint=% d&_wal_checkpoint_interval=% d",
	//	DefaultDBPath, DefaultSyncMode, DefaultWALAutoCheckpoint, DefaultWALCheckpointInterval)
	engine, err := xorm.NewEngine("sqlite3", DefaultDBPath)
	if err != nil {
		return nil, fmt.Errorf("error opening event manager db file (%v): %w", DefaultDBPath, err)
	}
	m.engine = engine

	// 3. Sync database structure and create indexes
	if err = m.engine.Sync2(new(TaskEvent)); err != nil {
		return nil, fmt.Errorf("error syncing database schema (TaskEvent): %w", err)
	}

	// 4. Start the snapshot goroutine in background if enabled
	if sc.Enabled {
		m.ticker = time.NewTicker(sc.Interval * time.Second)
		go m.backgroundBackup()
	}

	// 启动任务计数器清理
	// m.startTaskCounterCleanup()

	return m, nil
}

// Insert inserts a new event
func (m *EventManager) Insert(e *TaskEvent) error {
	session := m.engine.NewSession()
	defer session.Close()

	if err := session.Begin(); err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}

	// 1. Update event milestone based on its status
	// NOTE: started comes after dispatched
	if e.EventType == string(enum.TaskDispatched) {
		e.Milestone = MilestoneDispatched
	}
	if e.EventType == string(enum.TaskStarted) {
		e.Milestone = MilestoneStarted
	}

	// 2. Insert the latest event
	if _, err := session.Insert(e); err != nil {
		_ = session.Rollback()
		return fmt.Errorf("error inserting task event: %w", err)
	}

	// 2. Try to compact events
	if err := m.maybeCompactEvents(session, e); err != nil {
		_ = session.Rollback()
		return fmt.Errorf("error compacting events: %w", err)
	}

	return session.Commit()
}

// Iterate iterates over task events
func (m *EventManager) Iterate(tenantId, taskId string, fn func(e *TaskEvent) bool) error {
	var events []TaskEvent
	err := m.engine.Where("tenant_id = ? AND task_id = ?", tenantId, taskId).
		Asc("timestamp").Find(&events)
	if err != nil {
		return fmt.Errorf("error querying events: %w", err)
	}

	for _, event := range events {
		if !fn(&event) {
			break
		}
	}
	return nil
}

// Tasks retrieves ids of specified tenant's tasks
func (m *EventManager) Tasks(tenantId string) (ids []string, err error) {
	err = m.engine.Table(&TaskEvent{}).Where("tenant_id = ?", tenantId).
		Cols("task_id").Distinct("task_id").Find(&ids)
	if err != nil {
		return nil, fmt.Errorf("error querying tenant tasks: %w", err)
	}
	return ids, nil
}

// CountRunningTasks counts number of running tasks
func (m *EventManager) CountRunningTasks(tenantId string) (int, error) {
	var n, err = m.engine.Where("tenant_id = ?", tenantId).Distinct("task_id").Count(&TaskEvent{})
	return int(n), err
}

// Latest retrieves the latest event of a task
func (m *EventManager) Latest(tenantId, taskId string) (ev *TaskEvent, err error) {
	ev = new(TaskEvent)
	_, err = m.engine.Where("tenant_id = ? AND task_id = ?", tenantId, taskId).
		Desc("timestamp").Limit(1).Get(ev)
	if err != nil {
		return nil, fmt.Errorf("error querying event: %w", err)
	}
	if ev.ID <= 0 {
		return nil, fmt.Errorf("event not found")
	}
	return ev, nil
}

// Delete deletes task events
func (m *EventManager) Delete(tenantId, taskId string) error {
	session := m.engine.NewSession()
	defer session.Close()

	if err := session.Begin(); err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}

	if _, err := session.Where("tenant_id = ? AND task_id = ?", tenantId, taskId).Delete(&TaskEvent{}); err != nil {
		_ = session.Rollback()
		return fmt.Errorf("error deleting events: %w", err)
	}

	return session.Commit()
}

// Backup backs up local database and upload to object storage
func (m *EventManager) Backup() (string, error) {
	if !m.sc.Enabled {
		return "", nil
	}

	// 1. Create a temporary file
	tempFile, err := os.CreateTemp("", "sqlite_backup_*.db")
	if err != nil {
		return "", fmt.Errorf("error creating temp backup file: %w", err)
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()
	defer os.Remove(tempPath)

	// 2. Execute hot backup
	if err := m.hotBackup(tempPath); err != nil {
		return "", fmt.Errorf("error performing hot backup: %w", err)
	}

	// 3. Upload temporary file to object storage
	file, err := os.Open(tempPath)
	if err != nil {
		return "", fmt.Errorf("error opening backup file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("error getting file stats: %w", err)
	}

	key := m.snapshotKey(m.newSnapshotVersion())
	_, err = m.s3.PutObject(m.sc.Bucket, key, file, stat.Size(), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return "", fmt.Errorf("failed to write snapshot (%v) to s3: %w", key, err)
	}
	return key, nil
}

// hotBackup performs an online backup of the SQLite database without closing database connection
func (m *EventManager) hotBackup(destPath string) error {
	// 1. Open new database connection
	destDB, err := sql.Open("sqlite3", destPath)
	if err != nil {
		return fmt.Errorf("error opening destination database: %w", err)
	}
	defer destDB.Close()

	// 2. Start transaction
	tx, err := destDB.Begin()
	if err != nil {
		return fmt.Errorf("error starting backup transaction: %w", err)
	}
	defer tx.Rollback()

	// 3. Start backup
	_, err = tx.Exec(`ATTACH DATABASE ? AS src`, DefaultDBPath)
	if err != nil {
		return fmt.Errorf("error attaching source database: %w", err)
	}

	// 4. Backup in WAL mode
	_, err = tx.Exec(`
			BEGIN IMMEDIATE;
			SELECT sqlcipher_export('main', 'src.main');
			COMMIT;
		`)
	if err != nil {
		// Fallback to SQLCipher backup
		_, err = tx.Exec(`
				BEGIN IMMEDIATE;
				SELECT sqlitetoolbox_export('main', 'src.main');
				COMMIT;
			`)
		if err != nil {
			// Fallback to standard SQLite backup
			_, err = tx.Exec(`
					BEGIN IMMEDIATE;
					SELECT * FROM src.sqlite_master;
					COMMIT;
				`)
			if err != nil {
				return fmt.Errorf("error performing backup: %w", err)
			}
		}
	}

	// 5. Detach database
	_, err = tx.Exec(`DETACH DATABASE src`)
	if err != nil {
		return fmt.Errorf("error detaching source database: %w", err)
	}

	// 6. Commit transaction
	return tx.Commit()
}

// Close closes the event manager
func (m *EventManager) Close() error {
	close(m.stopped)
	return m.engine.Close()
}

// loadSnapshot tries to load the newest snapshot from object storage
// NOTE:
//   - Does nothing when no snapshot is available
//   - Report an error when there is a local event db file existing
func (m *EventManager) loadSnapshot() error {
	var (
		newest time.Time
		key    string
	)

	// 1. Find the newest version of snapshot file
	for i := 0; i < m.sc.MaxVersions; i++ {
		tmp := m.snapshotKey(i)
		info, err := m.s3.StatObject(m.sc.Bucket, tmp, minio.StatObjectOptions{})
		if err != nil {
			m.lg.Log(types.LevelInfo, "error", err.Error(), "key", tmp, "message", "snapshot not found")
			continue
		}
		if info.LastModified.After(newest) {
			newest = info.LastModified
			key = tmp
		}
	}
	if key == "" {
		m.lg.Log(types.LevelWarn, "message", "no available snapshot")
		return nil
	}
	m.lg.Log(types.LevelInfo, "key", key, "updated", newest, "message", "found the newest snapshot")

	// 2. Retrieve the newest snapshot file and read its content
	obj, err := m.s3.GetObject(m.sc.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("error fetching the newest snapshot file")
	}
	byt, err := io.ReadAll(obj)
	if err != nil {
		return fmt.Errorf("error reading the downloaded snapshot file")
	}

	// 3. Checks whether local database file already exists
	if _, err = os.Stat(DefaultDBPath); !(err != nil && strings.Contains(err.Error(), "no such file")) {
		return fmt.Errorf("found existing event db file, can not overwrite it with snapshot")
	}

	// 4. Write snapshot file content
	if err = os.WriteFile(DefaultDBPath, byt, os.ModePerm); err != nil {
		return fmt.Errorf("error writing snapshot file to db: %w", err)
	}
	m.lg.Log(types.LevelInfo, "key", key, "updated", newest, "message", "loaded the newest snapshot")
	return nil
}

// newSnapshotVersion generates a new snapshot version number
func (m *EventManager) newSnapshotVersion() int {
	tmp := m.sv
	m.sv += 1
	if m.sv > m.sc.MaxVersions-1 {
		m.sv = 0
	}
	return tmp
}

// snapshotKey generates a new snapshot key
func (m *EventManager) snapshotKey(v int) string {
	// e.g. cn-scheduler-1/events-db-snapshot-0
	return fmt.Sprintf("%v/%v-%v", m.sched, DefaultDBSnapshotPrefix, v)
}

// backgroundBackup takes snapshot and backup the file in background
func (m *EventManager) backgroundBackup() {
	for {
		select {
		case <-m.ticker.C:
			if key, err := m.Backup(); err != nil {
				m.lg.Log(types.LevelError, "error", err.Error(), "message", "error saving events db snapshot")
			} else {
				m.lg.Log(types.LevelInfo, "key", key, "message", "saved events db snapshot")
			}
		case <-m.stopped:
			m.ticker.Stop()
			return
		}
	}
}

// maybeCompactEvents compacts events by deleting database records to minimize storage usage
func (m *EventManager) maybeCompactEvents(session *xorm.Session, ev *TaskEvent) error {
	// 1. Delete outdated task status report event
	var deleted, err = session.Where("tenant_id = ? AND task_id = ? AND event_type = ? AND timestamp < ?",
		ev.TenantId, ev.TaskId, string(enum.ReportTaskStatus), ev.Timestamp).
		Delete(&TaskEvent{})
	if err != nil {
		m.lg.Log(types.LevelError, "error", err.Error(), "tenantId", ev.TenantId, "taskId", ev.TaskId,
			"message", "error performing db compaction (while removing outdated task status report event)")
		return err
	}

	m.lg.Log(types.LevelDebug, "deleted", deleted, "tenantId", ev.TenantId, "taskId", ev.TaskId,
		"message", "deleted outdated event")
	return nil
}
