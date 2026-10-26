package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// Jobs (expire_unpaid_holds, schedule_reminders, ...) arrive in phase 4.
func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("worker started")
	<-ctx.Done()
	log.Info("worker stopped")
}
