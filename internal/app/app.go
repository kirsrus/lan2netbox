// Package app — оркестратор рабочих горутин приложения.
//
// Запускает компоненты (sniffer, probe, netbox, web, zabbix) в отдельных
// горутинах, ожидает сигнал завершения (SIGINT/SIGTERM) или фатальную
// ошибку компонента и выполняет graceful shutdown.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// DefaultShutdownTimeout — максимальное время ожидания завершения
// компонентов при graceful shutdown.
const DefaultShutdownTimeout = 10 * time.Second

// Worker — рабочий компонент приложения, управляемый оркестратором.
type Worker interface {
	// Name возвращает имя компонента (используется в логах).
	Name() string
	// Run запускает компонент и блокируется до отмены ctx.
	// Возвращённая ошибка считается фатальной для всего приложения.
	Run(ctx context.Context) error
}

// App — оркестратор жизненного цикла компонентов.
type App struct {
	logger          *slog.Logger
	workers         []Worker
	shutdownTimeout time.Duration
}

// New создаёт оркестратор с указанными компонентами.
func New(logger *slog.Logger, workers ...Worker) *App {
	return &App{
		logger:          logger,
		workers:         workers,
		shutdownTimeout: DefaultShutdownTimeout,
	}
}

// SetShutdownTimeout переопределяет таймаут graceful shutdown.
func (a *App) SetShutdownTimeout(d time.Duration) *App {
	if d > 0 {
		a.shutdownTimeout = d
	}
	return a
}

// Run запускает все компоненты в отдельных горутинах и блокируется
// до одного из событий:
//
//   - сигнал SIGINT/SIGTERM — инициирует graceful shutdown;
//   - ошибка компонента — останавливает остальные компоненты
//     и возвращает ошибку.
func (a *App) Run(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	// Внутренний контекст с обработкой сигналов.
	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	a.logger.Info("оркестратор: запуск", "components", len(a.workers))

	errCh := make(chan error, len(a.workers))
	var wg sync.WaitGroup
	for _, w := range a.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.logger.Info("оркестратор: компонент запущен", "component", w.Name())
			if err := w.Run(sigCtx); err != nil {
				errCh <- fmt.Errorf("компонент %q: %w", w.Name(), err)
			}
			a.logger.Info("оркестратор: компонент завершил работу", "component", w.Name())
		}()
	}

	// Ожидание сигнала либо фатальной ошибки компонента.
	var runErr error
	select {
	case <-sigCtx.Done():
		a.logger.Info("оркестратор: получен сигнал завершения, graceful shutdown")
	case err := <-errCh:
		runErr = err
		a.logger.Error("оркестратор: аварийное завершение компонента", "error", err)
		cancel()
	}

	// Graceful shutdown: ждём завершения всех компонентов с таймаутом.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		a.logger.Info("оркестратор: все компоненты завершили работу")
	case <-time.After(a.shutdownTimeout):
		a.logger.Warn("оркестратор: таймаут завершения, выход без ожидания",
			"timeout", a.shutdownTimeout)
	}

	return runErr
}
