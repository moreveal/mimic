package scheduler

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDeterministicOrdering(t *testing.T) {
	s := New(time.Unix(0, 0), nil)
	var got []int
	s.Post(Timer, time.Second, func(context.Context) error { got = append(got, 2); return nil })
	s.Post(DOM, 0, func(context.Context) error { got = append(got, 1); return nil })
	if err := s.RunUntilIdle(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatal(got)
	}
}

func TestRunReadyDoesNotFastForwardTimers(t *testing.T) {
	s := New(time.Unix(0, 0), nil)
	called := false
	s.Post(Timer, time.Second, func(context.Context) error { called = true; return nil })
	if err := s.RunReady(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if called || !s.Now().Equal(time.Unix(0, 0)) {
		t.Fatalf("future timer ran during ready-task checkpoint: called=%v now=%v", called, s.Now())
	}
	if err := s.RunUntilIdle(context.Background(), 10); err != nil || !called {
		t.Fatalf("timer did not run when virtual time advanced: called=%v error=%v", called, err)
	}
}

func TestAdvanceMakesTimerReady(t *testing.T) {
	s := New(time.Unix(0, 0), nil)
	called := false
	s.Post(Timer, 25*time.Millisecond, func(context.Context) error { called = true; return nil })
	s.AdvanceBy(24 * time.Millisecond)
	if err := s.RunReady(context.Background(), 10); err != nil || called {
		t.Fatalf("timer became ready too early: called=%v error=%v", called, err)
	}
	s.AdvanceBy(time.Millisecond)
	if err := s.RunReady(context.Background(), 10); err != nil || !called {
		t.Fatalf("timer did not become ready: called=%v error=%v", called, err)
	}
}

func TestClockAdvancesWhileJavaScriptTaskRuns(t *testing.T) {
	s := New(time.Unix(0, 0), nil)
	var elapsed time.Duration
	s.Post(DOM, 0, func(context.Context) error {
		start := s.Now()
		for elapsed < 5*time.Millisecond {
			elapsed = s.Now().Sub(start)
		}
		return nil
	})
	if err := s.RunReady(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if elapsed < 5*time.Millisecond || s.Now().Before(time.Unix(0, 0).Add(elapsed)) {
		t.Fatalf("clock froze inside task: elapsed=%v now=%v", elapsed, s.Now())
	}
}

func TestClockAdvancesDuringMicrotaskCheckpoint(t *testing.T) {
	start := time.Unix(0, 0)
	var s *Scheduler
	var elapsed time.Duration
	s = New(start, func(context.Context) error {
		for elapsed < 5*time.Millisecond {
			elapsed = s.Now().Sub(start)
		}
		return nil
	})
	s.Post(DOM, 0, func(context.Context) error { return nil })
	if err := s.RunReady(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if elapsed < 5*time.Millisecond || s.Now().Before(start.Add(elapsed)) {
		t.Fatalf("clock froze during microtask checkpoint: elapsed=%v now=%v", elapsed, s.Now())
	}
}

func TestReadyScriptFetchStartsBeforePassiveResourceFetch(t *testing.T) {
	s := New(time.Unix(0, 0), nil)
	var got []string
	s.Post(ResourceLow, 0, func(context.Context) error { got = append(got, "image"); return nil })
	s.Post(ResourceScript, 0, func(context.Context) error { got = append(got, "script"); return nil })
	if err := s.RunReady(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"script", "image"}) {
		t.Fatalf("resource fetch-start priority mismatch: %v", got)
	}
}

func TestReadyPassiveResourceYieldsToBrowserTask(t *testing.T) {
	s := New(time.Unix(0, 0), nil)
	var got []string
	s.Post(ResourceLow, 0, func(context.Context) error { got = append(got, "passive"); return nil })
	s.Post(Timer, 0, func(context.Context) error { got = append(got, "timer"); return nil })
	if err := s.RunReady(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"timer", "passive"}) {
		t.Fatalf("passive resource did not yield to ready browser task: %v", got)
	}
}

func TestReadyTaskDoesNotMoveCanonicalClockBackwards(t *testing.T) {
	s := New(time.Unix(0, 0), nil)
	s.Post(DOM, 0, func(context.Context) error { return nil })
	s.AdvanceBy(time.Second)
	if err := s.RunReady(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if s.Now().Before(time.Unix(0, 0).Add(time.Second)) {
		t.Fatalf("canonical clock regressed to %v", s.Now())
	}
}

func TestConcurrentDriversNeverOverlapBrowserCallbacks(t *testing.T) {
	s := New(time.Unix(0, 0), nil)
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondStarted := make(chan struct{}, 1)
	s.Post(DOM, 0, func(context.Context) error {
		close(firstStarted)
		<-releaseFirst
		return nil
	})
	s.Post(Timer, 0, func(context.Context) error {
		secondStarted <- struct{}{}
		return nil
	})
	done := make(chan error, 2)
	go func() { done <- s.RunReady(context.Background(), 1) }()
	<-firstStarted
	go func() { done <- s.RunReady(context.Background(), 1) }()
	select {
	case <-secondStarted:
		t.Fatal("second browser callback overlapped the active event-loop turn")
	case <-time.After(20 * time.Millisecond):
	}
	close(releaseFirst)
	for range 2 {
		if err := <-done; err != nil && !strings.Contains(err.Error(), "task limit") {
			t.Fatal(err)
		}
	}
	select {
	case <-secondStarted:
	default:
		t.Fatal("serialized second callback did not run")
	}
}
