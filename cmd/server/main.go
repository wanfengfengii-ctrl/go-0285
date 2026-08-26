// Command server is the runnable entry point for the precast wall grouting
// quality backend. It accepts an event log path, snapshot path and logical
// clock mode, then serves the HTTP API with health/readiness reporting and
// graceful shutdown.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"precast-wall-grout-support-release/application"
	"precast-wall-grout-support-release/devices"
	"precast-wall-grout-support-release/httpapi"
	"precast-wall-grout-support-release/persistence"
)

func main() {
	var (
		addr     = flag.String("addr", ":8080", "HTTP listen address")
		logPath  = flag.String("event-log", "events.log", "append-only event log path")
		snapPath = flag.String("snapshot", "snapshot.bin", "atomic snapshot path")
		clockStr = flag.String("clock", string(application.ClockModeLogical), "logical|wall|manual")
	)
	flag.Parse()

	if err := run(*addr, *logPath, *snapPath, application.ClockMode(*clockStr)); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func run(addr, logPath, snapPath string, mode application.ClockMode) error {
	clock := newClock(mode)

	repo, err := persistence.NewFileRepository(logPath, snapPath)
	if err != nil {
		return err
	}

	svc := application.NewService(repo, clock, application.DefaultCatalog(), devices.NewProductionAdapter())
	server := httpapi.NewServer(svc)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on %s (clock=%s)", addr, mode)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		log.Printf("shutting down gracefully")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		_ = repo.Close()
		err := <-errCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// newClock selects a clock implementation from the configured mode.
func newClock(mode application.ClockMode) application.Clock {
	switch mode {
	case application.ClockModeWall:
		return application.NewWallClock()
	case application.ClockModeManual:
		return application.NewLogicalClock()
	default:
		return application.NewLogicalClock()
	}
}
