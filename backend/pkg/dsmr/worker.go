package dsmr

import (
	"log/slog"
	"context"
	"sync"
)

func worker(ctx context.Context,
	id int,
	jobs <-chan string,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Worker stopped", "worker_id", id)
			return
		case raw, ok := <-jobs:
			if !ok {
				slog.Info("Jobs channel closed", "worker_id", id)
				return
			}

			telegram, err := parseTelegram(raw)
			if err != nil {
				slog.Error("Failed to parse telegram", "worker_id", id, "error", err)
				continue
			}

			slog.Info("Processed telegram", "worker_id", id, "telegram", telegram)
		}
}
