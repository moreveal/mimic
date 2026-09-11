package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestCloseReleasesAndRejectsWork(t *testing.T) {
	s := New(time.Now(), nil)
	calls := 0
	s.Post(Timer, 0, func(context.Context) error { calls++; return nil })
	s.Close()
	s.Resume()
	if id := s.Post(Timer, 0, func(context.Context) error { calls++; return nil }); id != 0 {
		t.Fatalf("closed queue accepted work: %d", id)
	}
	if err := s.RunUntilIdle(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if calls != 0 || len(s.tasks) != 0 || len(s.byID) != 0 {
		t.Fatal("closed queue retained or ran callbacks")
	}
	s.Close()
}
