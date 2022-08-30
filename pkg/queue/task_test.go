package queue

import (
	"github.com/mykube-run/keel/pkg/database"
	"github.com/mykube-run/keel/pkg/entity"
	"github.com/rs/zerolog"
	"os"
	"testing"
	"time"
)

var (
	db     = database.NewMockDB()
	lg     = zerolog.New(os.Stdout)
	tenant = &entity.Tenant{
		Id:            10001,
		Uid:           "tenant-10001",
		Zone:          "default",
		Priority:      0,
		Name:          "Tenant-10001",
		Status:        0,
		ResourceQuota: entity.ResourceQuota{},
	}
)

func TestNewTaskCache(t *testing.T) {
	c := NewTaskQueue(db, &lg, tenant)
	if c.UserTasks == nil {
		t.Fatal("invalid tenant cache")
	}
}

func TestTaskCache_PopUserTasks(t *testing.T) {
	n := 3
	c := NewTaskQueue(db, &lg, tenant)

	// No user tasks for the first time
	{
		tasks, popped, err := c.PopUserTasks(n)
		if err != nil {
			t.Fatalf("error popping user tasks: %v", err)
		}
		if popped != 0 {
			t.Fatalf("expected popping 0 tasks, got %v", popped)
		}
		if len(tasks) != 0 {
			t.Fatalf("expected popping 0 tasks, got %v", len(tasks))
		}
		c.lg.Debug().Int("len", c.UserTasks.Len()).Msg("user tasks length")
	}

	// User tasks should be populated
	time.Sleep(time.Millisecond * 100)
	{
		tasks, popped, err := c.PopUserTasks(n)
		if err != nil {
			t.Fatalf("error popping user tasks: %v", err)
		}
		if popped != n {
			t.Fatalf("expected popping %v tasks, got %v", n, popped)
		}
		if len(tasks) != n {
			t.Fatalf("expected popping %v tasks, got %v", n, len(tasks))
		}
		c.lg.Debug().Int("len", c.UserTasks.Len()).Msg("user tasks length")
	}

}

func TestTaskCache_EnqueueUserTask(t *testing.T) {
	c := NewTaskQueue(db, &lg, tenant)
	c.FetchTasks()

	// No user tasks for the first time
	time.Sleep(time.Millisecond * 100)
	tasks, _, err := c.PopUserTasks(1)
	if err != nil {
		t.Fatalf("error popping user tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected popping 1 task, got %v", len(tasks))
	}

	task := tasks[0]
	if task.Id == 0 {
		t.Fatalf("invalid task with id %v", task.Id)
	}
	id := task.Id
	p := task.Priority
	c.EnqueueUserTask(task, 999)

	tasks2, _, err := c.PopUserTasks(1)
	if err != nil {
		t.Fatalf("error popping user tasks: %v", err)
	}
	if len(tasks2) != 1 {
		t.Fatalf("expected popping 1 task, got %v", len(tasks2))
	}
	task2 := tasks2[0]
	if task2.Priority-p != 999 {
		t.Fatalf("unexpected task priority: %v, expecting %v", task2.Priority, p+999)
	}
	if task2.Id != id {
		t.Fatalf("should got the same task")
	}
}

func TestTaskCache_PopulateTasks(t *testing.T) {
	c := NewTaskQueue(db, &lg, tenant)
	if c.UserTasks.Len() != 0 {
		t.Fatalf("expected user tasks length to be 0, got %v", c.UserTasks.Len())
	}

	c.FetchTasks()
	time.Sleep(time.Millisecond * 100)

	if c.UserTasks.Len() == 0 {
		t.Fatalf("expected user tasks length to be >0, got %v", c.UserTasks.Len())
	}
}
