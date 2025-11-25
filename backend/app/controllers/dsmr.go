package controllers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/erwindouna/p1ngry/pkg/dsmr"

	"github.com/gofiber/fiber/v2"
)

// Server side events
func SSESerializeReading(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	ctx := c.Context()
	done := ctx.Done()

	ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
		ch, unsubscribe := dsmr.Subscribe(0)
		defer unsubscribe()

		sendEvent := func(data dsmr.SmartMeterData) bool {
			payload, err := json.Marshal(data)
			if err != nil {
				slog.Error("sse: marshal telegram", "error", err)
				return true
			}

			if _, err := fmt.Fprintf(w, "event: telegram\ndata: %s\n\n", payload); err != nil {
				slog.Error("sse: write telegram", "error", err)
				return false
			}
			if err := w.Flush(); err != nil {
				slog.Error("sse: flush telegram", "error", err)
				return false
			}
			return true
		}

		if initial := dsmr.CurrentMetrics(); initial.Timestamp != "" {
			if !sendEvent(initial) {
				return
			}
		}

		heartbeat := time.NewTicker(30 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case <-done:
				return
			case <-heartbeat.C:
				if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			case data, ok := <-ch:
				if !ok {
					return
				}
				if !sendEvent(data) {
					return
				}
			}
		}
	})

	return nil
}
