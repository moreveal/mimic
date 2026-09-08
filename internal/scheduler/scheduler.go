package scheduler

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Source string

const (
	DOM        Source = "dom"
	Timer      Source = "timer"
	Network    Source = "network"
	Navigation Source = "navigation"
	Control    Source = "control"
	// PostedMessage represents the HTML posted message task source used by
	// MessagePort and related browser messaging primitives.
	PostedMessage Source = "posted-message"
	// ResourceScript and ResourceLow select transport-start work queued by DOM
	// mutations. Transport completions return through Network and do not inherit
	// this fetch-priority ordering.
	ResourceScript Source = "script-resource"
	ResourceLow    Source = "low-priority-resource"
)

type Callback func(context.Context) error
type Transition struct {
	Name   string
	TaskID uint64
	Source Source
	Due    time.Time
}
type task struct {
	id, sequence uint64
	due          time.Time
	source       Source
	callback     Callback
	cancelled    bool
}
type queue []*task

func (q queue) Len() int { return len(q) }
func (q queue) Less(i, j int) bool {
	if q[i].due.Equal(q[j].due) {
		return q[i].sequence < q[j].sequence
	}
	return q[i].due.Before(q[j].due)
}
func (q queue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *queue) Push(x any)   { *q = append(*q, x.(*task)) }
func (q *queue) Pop() any     { old := *q; n := len(old); x := old[n-1]; *q = old[:n-1]; return x }

type Scheduler struct {
	mu             sync.Mutex
	runMu          sync.Mutex
	now            time.Time
	seq            uint64
	tasks          queue
	byID           map[uint64]*task
	checkpoint     func(context.Context) error
	paused         bool
	observer       func(Transition)
	runningAt      time.Time
	runningBase    time.Time
	wake           chan struct{}
	executionScale float64
}

func New(start time.Time, checkpoint func(context.Context) error) *Scheduler {
	s := &Scheduler{now: start, byID: map[uint64]*task{}, checkpoint: checkpoint, wake: make(chan struct{}, 1), executionScale: 1}
	heap.Init(&s.tasks)
	return s
}
func (s *Scheduler) SetObserver(f func(Transition)) { s.mu.Lock(); s.observer = f; s.mu.Unlock() }
func (s *Scheduler) SetExecutionScale(scale float64) {
	if scale <= 0 {
		scale = 1
	}
	s.mu.Lock()
	s.executionScale = scale
	s.mu.Unlock()
}
func (s *Scheduler) Post(source Source, delay time.Duration, callback Callback) uint64 {
	s.mu.Lock()
	s.seq++
	t := &task{s.seq, s.seq, s.nowLocked().Add(delay), source, callback, false}
	heap.Push(&s.tasks, t)
	s.byID[t.id] = t
	observer := s.observer
	s.mu.Unlock()
	if observer != nil {
		observer(Transition{"posted", t.id, t.source, t.due})
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return t.id
}

// Wait blocks until external work is posted or the next scheduled task becomes
// ready in real elapsed time. It deliberately does not jump the canonical
// clock to a future timer: another browser agent may complete first.
func (s *Scheduler) Wait(ctx context.Context) error {
	s.mu.Lock()
	var timer *time.Timer
	var readyAt time.Time
	if len(s.tasks) > 0 {
		readyAt = s.tasks[0].due
		delay := readyAt.Sub(s.nowLocked())
		if delay <= 0 {
			s.mu.Unlock()
			return nil
		}
		scale := s.executionScale
		if scale <= 0 {
			scale = 1
		}
		timer = time.NewTimer(time.Duration(float64(delay) / scale))
	}
	s.mu.Unlock()
	if timer == nil {
		select {
		case <-s.wake:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	defer timer.Stop()
	select {
	case <-s.wake:
		return nil
	case <-timer.C:
		s.mu.Lock()
		if readyAt.After(s.now) {
			s.now = readyAt
		}
		s.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (s *Scheduler) Cancel(id uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t := s.byID[id]; t != nil {
		t.cancelled = true
		delete(s.byID, id)
	}
}
func (s *Scheduler) Pause()  { s.mu.Lock(); s.paused = true; s.mu.Unlock() }
func (s *Scheduler) Resume() { s.mu.Lock(); s.paused = false; s.mu.Unlock() }
func (s *Scheduler) RunUntilIdle(ctx context.Context, maxTasks int) error {
	return s.run(ctx, maxTasks, true)
}
func (s *Scheduler) RunReady(ctx context.Context, maxTasks int) error {
	return s.run(ctx, maxTasks, false)
}
func (s *Scheduler) run(ctx context.Context, maxTasks int, advance bool) error {
	// Multiple Go goroutines may wake or drive one browsing context (for
	// example navigation and a CDP clock pump), but Chrome still has one event
	// loop per agent. Serialize complete turns so callbacks and their microtask
	// checkpoints can never execute concurrently or overtake queued work.
	s.runMu.Lock()
	defer s.runMu.Unlock()
	var taskErrors []error
	for i := 0; i < maxTasks; i++ {
		s.mu.Lock()
		if s.paused || len(s.tasks) == 0 {
			s.mu.Unlock()
			return errors.Join(taskErrors...)
		}
		t := s.popNextLocked(advance)
		if t == nil {
			s.mu.Unlock()
			return errors.Join(taskErrors...)
		}
		delete(s.byID, t.id)
		if t.due.After(s.now) {
			s.now = t.due
		}
		observer := s.observer
		s.mu.Unlock()
		if t.cancelled {
			continue
		}
		if observer != nil {
			observer(Transition{"start", t.id, t.source, t.due})
		}
		if err := ctx.Err(); err != nil {
			return errors.Join(append(taskErrors, err)...)
		}
		s.mu.Lock()
		s.runningBase = s.now
		s.runningAt = time.Now()
		s.mu.Unlock()
		callbackErr := t.callback(ctx)
		if callbackErr != nil {
			if observer != nil {
				observer(Transition{"error", t.id, t.source, t.due})
			}
			taskErrors = append(taskErrors, fmt.Errorf("%s task: %w", t.source, callbackErr))
		}
		if s.checkpoint != nil {
			if observer != nil {
				observer(Transition{"microtaskCheckpoint", t.id, t.source, t.due})
			}
			if err := s.checkpoint(ctx); err != nil {
				taskErrors = append(taskErrors, fmt.Errorf("microtask checkpoint: %w", err))
			}
		}
		// A microtask checkpoint is part of the same browser event-loop turn as
		// the task callback. Keep the canonical monotonic clock live until every
		// Promise job has run: JavaScript may observe time from a microtask, and
		// freezing it here can turn an otherwise finite polling chain into an
		// infinite one.
		s.mu.Lock()
		if !s.runningAt.IsZero() {
			s.now = s.runningBase.Add(time.Duration(float64(time.Since(s.runningAt)) * s.executionScale))
			s.runningAt = time.Time{}
			s.runningBase = time.Time{}
		}
		s.mu.Unlock()
		if observer != nil {
			observer(Transition{"end", t.id, t.source, t.due})
		}
	}
	taskErrors = append(taskErrors, fmt.Errorf("scheduler task limit %d exceeded", maxTasks))
	return errors.Join(taskErrors...)
}

func (s *Scheduler) popNextLocked(advance bool) *task {
	if len(s.tasks) == 0 {
		return nil
	}
	if s.tasks[0].due.After(s.now) {
		if !advance {
			return nil
		}
		s.now = s.tasks[0].due
	}
	best := -1
	for index, candidate := range s.tasks {
		if candidate.due.After(s.now) {
			continue
		}
		if best < 0 || taskBefore(candidate, s.tasks[best]) {
			best = index
		}
	}
	if best < 0 {
		return nil
	}
	return heap.Remove(&s.tasks, best).(*task)
}

func taskBefore(left, right *task) bool {
	leftPriority, rightPriority := sourcePriority(left.source), sourcePriority(right.source)
	if leftPriority != rightPriority {
		return leftPriority < rightPriority
	}
	if !left.due.Equal(right.due) {
		return left.due.Before(right.due)
	}
	return left.sequence < right.sequence
}

func sourcePriority(source Source) int {
	if source == ResourceScript {
		return -1
	}
	if source == ResourceLow {
		return 1
	}
	return 0
}
func (s *Scheduler) nowLocked() time.Time {
	if !s.runningAt.IsZero() {
		return s.runningBase.Add(time.Duration(float64(time.Since(s.runningAt)) * s.executionScale))
	}
	return s.now
}
func (s *Scheduler) Now() time.Time { s.mu.Lock(); defer s.mu.Unlock(); return s.nowLocked() }
func (s *Scheduler) AdvanceBy(delta time.Duration) {
	if delta <= 0 {
		return
	}
	s.mu.Lock()
	s.now = s.now.Add(delta)
	s.mu.Unlock()
}
