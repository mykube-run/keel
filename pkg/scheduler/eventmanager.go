package scheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/minio/minio-go"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/mykube-run/keel/pkg/enum"
	"github.com/mykube-run/keel/pkg/types"
	"go.etcd.io/bbolt"
	"io"
	"os"
	"strings"
	"time"
)

var (
	DefaultDBPath               = "./events.db"
	DefaultDBSnapshotPrefix     = "events-db-snapshot"
	DefaultEventCompactDuration = 60 // 60 seconds
	DefaultTaskCounterTTL       = 30 // 30 seconds
)

var BoltDBOption = &bbolt.Options{
	Timeout:      time.Second,
	NoGrowSync:   false,
	FreelistType: bbolt.FreelistArrayType,
}

const (
	MilestoneKeyPrefix  = "milestone-"
	MilestoneLatest     = MilestoneKeyPrefix + "latest"
	MilestoneDispatched = MilestoneKeyPrefix + "dispatched"
	MilestoneStarted    = MilestoneKeyPrefix + "started"
)

const (
	TaskDispatched = "TaskDispatched"
)

type TaskEvent struct {
	EventType string          `json:"eventType"`
	WorkerId  string          `json:"workerId"`
	TenantId  string          `json:"tenantId"`
	TaskId    string          `json:"taskId"`
	Timestamp time.Time       `json:"timestamp"`
	Value     json.RawMessage `json:"value"`
}

func (ev *TaskEvent) Key() []byte {
	return []byte(fmt.Sprintf("%v", ev.Timestamp.UnixNano()))
}

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

func NewEventFromTask(typ string, t *entity.Task) *TaskEvent {
	return &TaskEvent{
		EventType: typ,
		WorkerId:  "",
		TenantId:  t.TenantId,
		TaskId:    t.Uid,
		Timestamp: time.Now(),
		Value:     nil,
	}
}

type EventManager struct {
	db    *bbolt.DB
	sc    config.SnapshotConfig
	s3    *minio.Client
	sv    int // snapshot version
	lg    types.Logger
	sched string
}

func NewEventManager(sc config.SnapshotConfig, schedulerId string, lg types.Logger) (*EventManager, error) {
	m := &EventManager{
		sc:    sc,
		lg:    lg,
		sched: schedulerId,
	}

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

	db, err := bbolt.Open(DefaultDBPath, os.ModePerm, BoltDBOption)
	if err != nil {
		return nil, fmt.Errorf("error opening event manager db file (%v): %w", DefaultDBPath, err)
	}
	m.db = db

	if sc.Enabled {
		go m.backgroundBackup()
	}
	return m, nil
}

func (m *EventManager) CreateTenantBucket(tenantId string) {
	_ = m.db.Update(func(tx *bbolt.Tx) error {
		_, _ = tx.CreateBucketIfNotExists([]byte(tenantId))
		return nil
	})
}

func (m *EventManager) Insert(e *TaskEvent) error {
	tx, err := m.db.Begin(true)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	tenant := tx.Bucket([]byte(e.TenantId))
	if tenant == nil {
		return fmt.Errorf("tenant bucket does not exist: %v", e.TenantId)
	}
	task, err := tenant.CreateBucketIfNotExists([]byte(e.TaskId))
	if err != nil {
		return fmt.Errorf("error creating bucket for task (%v): %w", e.TaskId, err)
	}

	key := e.Key()
	byt, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("error marshalling task event: %w", err)
	}
	if err = task.Put(key, byt); err != nil {
		return fmt.Errorf("error inserting task event: %w", err)
	}

	// Get the latest event, maybe compacted later
	latestKey := task.Get([]byte(MilestoneLatest))

	// Store event's key in MilestoneLatest
	if err = task.Put([]byte(MilestoneLatest), key); err != nil {
		return fmt.Errorf("error inserting milestone event: %w", err)
	}
	// Try to compact events
	m.maybeCompactEvents(task, latestKey, e)

	// Store event's key in MilestoneDispatched
	if e.EventType == TaskDispatched {
		if err = task.Put([]byte(MilestoneDispatched), key); err != nil {
			return fmt.Errorf("error inserting milestone event: %w", err)
		}
	}
	// Store event's key in MilestoneStarted
	if e.EventType == string(enum.TaskStarted) {
		if err = task.Put([]byte(MilestoneStarted), key); err != nil {
			return fmt.Errorf("error inserting milestone event: %w", err)
		}
	}
	return tx.Commit()
}

func (m *EventManager) Iterate(tenantId, taskId string, fn func(e *TaskEvent) bool) error {
	return m.db.View(func(tx *bbolt.Tx) error {
		tenant := tx.Bucket([]byte(tenantId))
		if tenant == nil {
			m.lg.Log(types.LevelWarn, "tenantId", tenantId, "message", "tenant db is empty")
			return nil
		}

		task := tenant.Bucket([]byte(taskId))
		if task == nil {
			// m.lg.Log(types.LevelInfo, "taskId", taskId, "message", "task db is empty (can be ignored)")
			return nil
		}

		c := task.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if m.isMilestone(k) {
				continue
			}

			var e TaskEvent
			if err := json.Unmarshal(v, &e); err != nil {
				return err
			}
			if !fn(&e) {
				return nil
			}
		}
		return nil
	})
}

func (m *EventManager) Tasks(tenantId string) (ids []string, err error) {
	fn := func(k, v []byte) error {
		ids = append(ids, string(k))
		return nil
	}
	err = m.db.View(func(tx *bbolt.Tx) error {
		tenant := tx.Bucket([]byte(tenantId))
		if tenant == nil {
			return nil
		}
		return tenant.ForEach(fn)
	})
	return
}

func (m *EventManager) CountRunningTasks(tenantId string) (n int, err error) {
	ids, err := m.Tasks(tenantId)
	if err != nil {
		return
	}
	n = len(ids)
	return
}

func (m *EventManager) Latest(tenantId, taskId string) (ev *TaskEvent, err error) {
	err = m.db.View(func(tx *bbolt.Tx) error {
		tenant := tx.Bucket([]byte(tenantId))
		if tenant == nil {
			return nil
		}

		task := tenant.Bucket([]byte(taskId))
		if task == nil {
			return nil
		}

		var err1 error
		ev, err1 = m.getEventByKey(task, task.Get([]byte(MilestoneLatest)))
		return err1
	})

	if err != nil || ev == nil {
		err = fmt.Errorf("the latest event does not exist (%v)", err)
	}
	return
}

func (m *EventManager) Delete(tenantId, taskId string) error {
	tx, err := m.db.Begin(true)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	tenant := tx.Bucket([]byte(tenantId))
	if tenant == nil {
		return nil
	}

	if err = tenant.DeleteBucket([]byte(taskId)); err != nil {
		return fmt.Errorf("error deleting task bucket: %w", err)
	}
	return tx.Commit()
}

func (m *EventManager) Backup() (string, error) {
	if !m.sc.Enabled {
		return "", nil
	}

	buf := new(bytes.Buffer)
	err := m.db.View(func(tx *bbolt.Tx) error {
		_, err := tx.WriteTo(buf)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("error writing db to writer: %w", err)
	}

	key := m.snapshotKey(m.newSnapshotVersion())
	_, err = m.s3.PutObject(m.sc.Bucket, key, buf, int64(buf.Len()), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return "", fmt.Errorf("failed to write snapshot (%v) to s3: %w", key, err)
	}
	return key, nil
}

func (m *EventManager) isMilestone(k []byte) bool {
	return strings.HasPrefix(string(k), MilestoneKeyPrefix)
}

// loadSnapshot tries to load the newest snapshot from object storage
// NOTE:
//   - Does nothing when no snapshot available
//   - Report an error when there is a local event db file existing
func (m *EventManager) loadSnapshot() error {
	var (
		newest time.Time
		key    string
	)
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

	obj, err := m.s3.GetObject(m.sc.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("error fetching the newest snapshot file")
	}

	byt, err := io.ReadAll(obj)
	if err != nil {
		return fmt.Errorf("error reading the downloaded snapshot file")
	}

	if _, err = os.Stat(DefaultDBPath); !(err != nil && strings.Contains(err.Error(), "no such file")) {
		return fmt.Errorf("found existing event db file, can not overwrite it with snapshot")
	}

	if err = os.WriteFile(DefaultDBPath, byt, os.ModePerm); err != nil {
		return fmt.Errorf("error writing snapshot file to db: %w", err)
	}
	m.lg.Log(types.LevelInfo, "key", key, "updated", newest, "message", "loaded the newest snapshot")
	return nil
}

func (m *EventManager) newSnapshotVersion() int {
	tmp := m.sv
	m.sv += 1
	if m.sv > m.sc.MaxVersions-1 {
		m.sv = 0
	}
	return tmp
}

func (m *EventManager) latestEvent(bucket *bbolt.Bucket) (ev *TaskEvent, err error) {
	return m.getEventByKey(bucket, bucket.Get([]byte(MilestoneLatest)))
}

func (m *EventManager) getEventByKey(bucket *bbolt.Bucket, key []byte) (ev *TaskEvent, err error) {
	if bucket == nil || key == nil {
		return nil, fmt.Errorf("nil bucket or key while getting event by key")
	}
	byt := bucket.Get(key)
	if byt == nil {
		return nil, fmt.Errorf("nil event")
	}

	ev = new(TaskEvent)
	if err = json.Unmarshal(byt, ev); err != nil {
		return nil, err
	}
	return ev, nil
}

func (m *EventManager) maybeCompactEvents(bucket *bbolt.Bucket, key []byte, ev *TaskEvent) {
	latest, _ := m.getEventByKey(bucket, key)
	if latest == nil {
		return
	}

	switch latest.EventType {
	case string(enum.ReportTaskStatus):
		if ev.Timestamp.After(latest.Timestamp) &&
			ev.Timestamp.Sub(latest.Timestamp).Seconds() >= float64(DefaultEventCompactDuration) {
			if err := bucket.Delete(ev.Key()); err != nil {
				m.lg.Log(types.LevelError, "error", err.Error(), "key", key, "message", "error performing db compaction (while removing outdated task status report event)")
			}
		}
	}
}

func (m *EventManager) snapshotKey(v int) string {
	// e.g. cn-scheduler-1/events-db-snapshot-0
	return fmt.Sprintf("%v/%v-%v", m.sched, DefaultDBSnapshotPrefix, v)
}

func (m *EventManager) backgroundBackup() {
	tick := time.NewTicker(m.sc.Interval * time.Second)
	for {
		select {
		case <-tick.C:
			if key, err := m.Backup(); err != nil {
				m.lg.Log(types.LevelError, "error", err.Error(), "message", "error saving events db snapshot")
			} else {
				m.lg.Log(types.LevelInfo, "key", key, "message", "saved events db snapshot")
			}
		}
	}
}
