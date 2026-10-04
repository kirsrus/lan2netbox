package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

// stubWorker — тестовый воркер, блокирующийся до отмены контекста.
type stubWorker struct {
	name    string
	started atomic.Bool
}

func (s *stubWorker) Name() string { return s.name }

func (s *stubWorker) Run(ctx context.Context) error {
	s.started.Store(true)
	<-ctx.Done()
	return nil
}

// failingWorker — воркер, сразу возвращающий фатальную ошибку.
type failingWorker struct {
	err error
}

func (f *failingWorker) Name() string { return "failing" }

func (f *failingWorker) Run(_ context.Context) error { return f.err }

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// waitStarted ожидает запуска всех воркеров (с таймаутом).
func waitStarted(t *testing.T, ws ...*stubWorker) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		all := true
		for _, w := range ws {
			if !w.started.Load() {
				all = false
				break
			}
		}
		if all {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("воркеры не запустились за отведённое время")
}

func TestRunStopsWorkersOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	w1 := &stubWorker{name: "w1"}
	w2 := &stubWorker{name: "w2"}

	done := make(chan error, 1)
	go func() { done <- New(testLogger(), w1, w2).Run(ctx) }()
	waitStarted(t, w1, w2)

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run(): err = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run(): не завершился после отмены контекста")
	}
}

func TestRunReturnsWorkerError(t *testing.T) {
	want := errors.New("boom")
	w := &failingWorker{err: want}

	done := make(chan error, 1)
	go func() { done <- New(testLogger(), w).Run(context.Background()) }()

	select {
	case err := <-done:
		if !errors.Is(err, want) {
			t.Fatalf("Run(): err = %v, want содержащий %v", err, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run(): не вернул ошибку компонента")
	}
}
