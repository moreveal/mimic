package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestInlineClockIncludesBodyAndCheckpointWithoutDequeuing(t *testing.T) {
	var s *Scheduler
	var bodyEnd, checkpointEnd time.Time
	sentinel := errors.New("body failed")
	s = New(time.Unix(0, 0), func(context.Context) error {
		start := s.Now()
		time.Sleep(5 * time.Millisecond)
		checkpointEnd = s.Now()
		if checkpointEnd.Sub(start) < 5*time.Millisecond {
			t.Error("clock stopped in checkpoint")
		}
		return nil
	})
	s.Post(Timer, 0, func(context.Context) error { t.Error("inline turn dequeued a timer"); return nil })
	err := s.RunInline(context.Background(), func(context.Context) error {
		start := s.Now()
		time.Sleep(5 * time.Millisecond)
		bodyEnd = s.Now()
		if bodyEnd.Sub(start) < 5*time.Millisecond {
			t.Error("clock stopped in body")
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) || !checkpointEnd.After(bodyEnd) || s.Now().Before(checkpointEnd) {
		t.Fatalf("turn completion: err=%v body=%v checkpoint=%v now=%v", err, bodyEnd, checkpointEnd, s.Now())
	}
	end := s.Now()
	time.Sleep(time.Millisecond)
	if s.Now() != end {
		t.Fatal("execution clock leaked beyond turn")
	}
}
