package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/pibot/pibot/internal/ai"
	"github.com/pibot/pibot/internal/executor"
)

// ActionType represents the type of action a task performs.
type ActionType string

const (
	ActionShell ActionType = "shell"
	ActionAI    ActionType = "ai"
)

// maxHistoryPerTask is the maximum number of run records kept per task.
const maxHistoryPerTask = 50

// Task represents a scheduled task.
type Task struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Schedule   string     `json:"schedule"` // cron expression or @every <duration>
	ActionType ActionType `json:"action_type"`
	Command    string     `json:"command"` // shell command or AI prompt
	Enabled    bool       `json:"enabled"`
	LastRun    *time.Time `json:"last_run,omitempty"`
	NextRun    time.Time  `json:"next_run"`
	CreatedAt  time.Time  `json:"created_at"`
}

// RunRecord represents a single execution record for a task.
type RunRecord struct {
	TaskID    string    `json:"task_id"`
	StartedAt time.Time `json:"started_at"`
	Duration  string    `json:"duration"`
	Output    string    `json:"output"`
	Error     string    `json:"error,omitempty"`
	Success   bool      `json:"success"`
}

// Scheduler manages and runs scheduled tasks.
type Scheduler struct {
	exec    *executor.Executor
	chat    *ai.ChatSession
	parser  cron.Parser
	tasks   map[string]*Task
	history map[string][]RunRecord
	mu      sync.RWMutex
}

// NewScheduler creates a new Scheduler with the given executor.
// Call SetChatSession to enable AI action support.
func NewScheduler(exec *executor.Executor) *Scheduler {
	return &Scheduler{
		exec: exec,
		// Support standard 5-field cron plus @every and @daily etc.
		parser:  cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
		tasks:   make(map[string]*Task),
		history: make(map[string][]RunRecord),
	}
}

// SetChatSession configures the AI chat session used by AI action tasks.
func (s *Scheduler) SetChatSession(chat *ai.ChatSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chat = chat
}

// Start begins the scheduler tick loop. It blocks until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	log.Println("[scheduler] started")
	for {
		select {
		case <-ctx.Done():
			log.Println("[scheduler] stopped")
			return
		case now := <-ticker.C:
			s.tick(ctx, now)
		}
	}
}

// tick checks all enabled tasks and fires any that are due.
func (s *Scheduler) tick(ctx context.Context, now time.Time) {
	s.mu.Lock()
	var due []*Task
	for _, t := range s.tasks {
		if t.Enabled && !t.NextRun.IsZero() && !now.Before(t.NextRun) {
			due = append(due, t)
			// Advance NextRun immediately to prevent double-firing during long runs.
			next := s.nextAfter(t.Schedule, now)
			t.NextRun = next
		}
	}
	s.mu.Unlock()

	for _, t := range due {
		go s.run(ctx, t)
	}
}

// Add validates and registers a new task, returning its assigned ID.
func (s *Scheduler) Add(task *Task) error {
	if task.Name == "" {
		return fmt.Errorf("task name is required")
	}
	if task.Schedule == "" {
		return fmt.Errorf("task schedule is required")
	}
	if task.Command == "" {
		return fmt.Errorf("task command is required")
	}
	if task.ActionType != ActionShell && task.ActionType != ActionAI {
		return fmt.Errorf("action_type must be %q or %q", ActionShell, ActionAI)
	}

	nextRun, err := s.parseNextRun(task.Schedule)
	if err != nil {
		return fmt.Errorf("invalid schedule %q: %w", task.Schedule, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	task.ID = uuid.New().String()
	task.CreatedAt = time.Now()
	task.NextRun = nextRun
	s.tasks[task.ID] = task
	s.history[task.ID] = nil

	log.Printf("[scheduler] added task %q (id=%s, schedule=%s)", task.Name, task.ID, task.Schedule)
	return nil
}

// Remove deletes a task by ID.
func (s *Scheduler) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return fmt.Errorf("task not found: %s", id)
	}
	delete(s.tasks, id)
	delete(s.history, id)
	log.Printf("[scheduler] removed task id=%s", id)
	return nil
}

// Update replaces a task's editable fields and recomputes NextRun.
func (s *Scheduler) Update(id string, updated *Task) error {
	if updated.Name == "" {
		return fmt.Errorf("task name is required")
	}
	if updated.Schedule == "" {
		return fmt.Errorf("task schedule is required")
	}
	if updated.Command == "" {
		return fmt.Errorf("task command is required")
	}
	if updated.ActionType != ActionShell && updated.ActionType != ActionAI {
		return fmt.Errorf("action_type must be %q or %q", ActionShell, ActionAI)
	}

	nextRun, err := s.parseNextRun(updated.Schedule)
	if err != nil {
		return fmt.Errorf("invalid schedule %q: %w", updated.Schedule, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.tasks[id]
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}

	existing.Name = updated.Name
	existing.Schedule = updated.Schedule
	existing.ActionType = updated.ActionType
	existing.Command = updated.Command
	existing.Enabled = updated.Enabled
	existing.NextRun = nextRun

	log.Printf("[scheduler] updated task %q (id=%s)", existing.Name, id)
	return nil
}

// Get returns a copy of a task by ID.
func (s *Scheduler) Get(id string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	cp := *t
	return &cp, nil
}

// List returns copies of all registered tasks.
func (s *Scheduler) List() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		cp := *t
		result = append(result, &cp)
	}
	return result
}

// GetHistory returns the run history for a task.
func (s *Scheduler) GetHistory(taskID string) []RunRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := s.history[taskID]
	if records == nil {
		return []RunRecord{}
	}
	cp := make([]RunRecord, len(records))
	copy(cp, records)
	return cp
}

// RunNow triggers a task immediately (ignoring its schedule).
func (s *Scheduler) RunNow(id string) error {
	s.mu.RLock()
	t, ok := s.tasks[id]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}
	go s.run(context.Background(), t)
	return nil
}

// run executes the task and records the result.
func (s *Scheduler) run(ctx context.Context, t *Task) {
	start := time.Now()
	log.Printf("[scheduler] running task %q (id=%s, type=%s)", t.Name, t.ID, t.ActionType)

	output, err := runAction(ctx, t, s.exec, s.chat)

	duration := time.Since(start)
	now := time.Now()

	rec := RunRecord{
		TaskID:    t.ID,
		StartedAt: start,
		Duration:  duration.String(),
		Output:    output,
		Success:   err == nil,
	}
	if err != nil {
		rec.Error = err.Error()
		log.Printf("[scheduler] task %q FAILED in %s: %v", t.Name, duration, err)
	} else {
		log.Printf("[scheduler] task %q completed in %s", t.Name, duration)
	}

	s.mu.Lock()
	t.LastRun = &now
	records := s.history[t.ID]
	records = append(records, rec)
	if len(records) > maxHistoryPerTask {
		records = records[len(records)-maxHistoryPerTask:]
	}
	s.history[t.ID] = records
	s.mu.Unlock()
}

// parseNextRun computes the next run time for a schedule starting from now.
func (s *Scheduler) parseNextRun(schedule string) (time.Time, error) {
	sched, err := s.parser.Parse(schedule)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(time.Now()), nil
}

// nextAfter returns the next run time after `after` for a schedule.
func (s *Scheduler) nextAfter(schedule string, after time.Time) time.Time {
	sched, err := s.parser.Parse(schedule)
	if err != nil {
		return time.Time{}
	}
	return sched.Next(after)
}
