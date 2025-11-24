package dsmr

import (
	"context"
	"strings"
	"strconv"
	"time"
	"log/slog"
)

type Telegram struct {
	Timestamp    time.Time
	PowerUsageW  int
	PowerReturnW int
	Raw          string
}

func parseTelegram(raw string) (*Telegram, error) {
	lines := strings.Split(raw, "\n")

	telegram := &Telegram{
		Raw: raw,
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "0-0:1.0.0") {
			timestampStr := strings.Split(line, "(")[1]
			timestampStr = strings.Split(timestampStr, "W")[0]
			timestamp, err := time.Parse("060102150405S", timestampStr)
			if err != nil {
				return nil, err
			}
			telegram.Timestamp = timestamp
		} else if strings.HasPrefix(line, "1-0:1.7.0") {
			powerUsageStr := strings.Split(line, "(")[1]
			powerUsageStr = strings.Split(powerUsageStr, "*")[0]
			powerUsageW, err := strconv.Atoi(strings.Split(powerUsageStr, ".")[0])
			if err != nil {
				return nil, err
			}
			telegram.PowerUsageW = powerUsageW
		}

	return telegram, nil
}	

func handleTelegram(ctx context.Context, telegram *Telegram) error {

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		slog.Info("Handling telegram", "timestamp", telegram.Timestamp, "power_usage_w", telegram.PowerUsageW, "power_return_w", telegram.PowerReturnW)
		return nil
	}
}