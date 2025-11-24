package dsmr

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

func startP1Reader(ctx context.Context, jobs chan<- string, wg *sync.WaitGroup) error {
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer close(jobs)

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("DSMR reader stopped")
				return
			case <-ticker.C:
				slog.Info("Handling new telegram")
				rawTelegram := "/ISk5\2MT382-1000\n0-0:1.0.0(230623150405S)\n1-0:1.7.0(00123.456*kW)\n1-0:2.7.0(00000.000*kW)\n!"
				jobs <- rawTelegram
			}
		}
	}()

	return nil
}
