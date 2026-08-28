package dsmr

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/roaldnefs/go-dsmr"
	"github.com/tarm/serial"
)

type Mode string

const (
	ModeSerial Mode = "serial"
	ModeMock   Mode = "mock"
)

const defaultSubscriberBuffer = 32

type Config struct {
	Mode   Mode
	Serial SerialConfig
	Mock   MockConfig
}

type SerialConfig struct {
	Device      string
	BaudRate    int
	ReadTimeout time.Duration
}

type MockConfig struct {
	Interval  time.Duration
	Repeat    bool
	Telegrams []string
}

type SmartMeterData struct {
	Timestamp               string
	CurrentPowerConsumption float64
	CurrentPowerGeneration  float64
	InstVoltL1              float64
	InstVoltL2              float64
	InstVoltL3              float64
	InstCurrentL1           float64
	InstCurrentL2           float64
	InstCurrentL3           float64
	GasDelivered            float64
	PowerDeliveredTariff1   float64
	PowerDeliveredTariff2   float64
	PowerGeneratedTariff1   float64
	PowerGeneratedTariff2   float64
}

type Handler interface {
	Handle(ctx context.Context, data SmartMeterData)
}

type HandlerFunc func(ctx context.Context, data SmartMeterData)

func (f HandlerFunc) Handle(ctx context.Context, data SmartMeterData) {
	f(ctx, data)
}

var (
	metricsMu        sync.RWMutex
	currentMetrics   SmartMeterData
	handlersMu       sync.RWMutex
	externalHandlers []Handler
	runMu            sync.Mutex
	runCancel        context.CancelFunc
	subscribersMu    sync.RWMutex
	subscribers      = make(map[uint64]chan SmartMeterData)
	subscriberSeq    atomic.Uint64
)

func RegisterHandler(handler Handler) {
	if handler == nil {
		return
	}
	handlersMu.Lock()
	externalHandlers = append(externalHandlers, handler)
	handlersMu.Unlock()
}

// Subscribe returns a buffered channel receiving telegram metrics and an
// unsubscribe function that must be called when the consumer is finished.
// A neat pub/sub pattern.
func Subscribe(buffer int) (<-chan SmartMeterData, func()) {
	if buffer <= 0 {
		buffer = defaultSubscriberBuffer
	}

	ch := make(chan SmartMeterData, buffer)
	id := subscriberSeq.Add(1)

	subscribersMu.Lock()
	subscribers[id] = ch
	subscribersMu.Unlock()

	unsubscribe := func() {
		subscribersMu.Lock()
		sub, ok := subscribers[id]
		if ok {
			delete(subscribers, id)
		}
		subscribersMu.Unlock()

		if ok {
			close(sub)
		}
	}

	return ch, unsubscribe
}

func dispatchHandlers(ctx context.Context, data SmartMeterData) {
	handlersMu.RLock()
	handlers := append([]Handler(nil), externalHandlers...)
	handlersMu.RUnlock()
	for _, handler := range handlers {
		handler.Handle(ctx, data)
	}

	subscribersMu.RLock()
	subs := make([]subscriberEntry, 0, len(subscribers))
	for id, ch := range subscribers {
		subs = append(subs, subscriberEntry{id: id, ch: ch})
	}
	subscribersMu.RUnlock()

	for _, sub := range subs {
		func(entry subscriberEntry) {
			defer func() {
				if r := recover(); r != nil {
					subscribersMu.Lock()
					if existing, ok := subscribers[entry.id]; ok && existing == entry.ch {
						delete(subscribers, entry.id)
					}
					subscribersMu.Unlock()
				}
			}()

			select {
			case entry.ch <- data:
			default:
			}
		}(sub)
	}
}

type subscriberEntry struct {
	id uint64
	ch chan SmartMeterData
}

func CurrentMetrics() SmartMeterData {
	metricsMu.RLock()
	defer metricsMu.RUnlock()
	return currentMetrics
}

func Start(cfg Config, handler Handler) {
	runMu.Lock()
	defer runMu.Unlock()

	slog.Info("Starting new reader")

	if runCancel != nil {
		runCancel()
		runCancel = nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	runCancel = cancel
	go Run(ctx, cfg, handler)
}

func Stop() {
	runMu.Lock()
	defer runMu.Unlock()

	slog.Info("Stopping reader. Most likely for a config refresh. ;)")

	if runCancel != nil {
		runCancel()
		runCancel = nil
	}
}

func RunReader() {
	cfg := ConfigFromEnv()
	if cfg.Mode == ModeMock {
		slog.Warn("dsmr: running in mock mode")
	}

	Start(cfg, HandlerFunc(func(ctx context.Context, data SmartMeterData) {
		metricsMu.Lock()
		currentMetrics = data
		metricsMu.Unlock()

		if boolFromEnv("DEBUG_LOGGING", false) {
			slog.Info("dsmr: telegram received", "telegram", data)
		}
		dispatchHandlers(ctx, data)
	}))
}

func Run(ctx context.Context, cfg Config, handler Handler) {
	if handler == nil {
		handler = HandlerFunc(func(context.Context, SmartMeterData) {})
	}

	source, err := newSource(cfg)
	if err != nil {
		slog.Error("dsmr: failed to initialise source", "error", err)
		return
	}
	defer source.Close()

	for {
		telegramRaw, err := source.Next(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				slog.Info("dsmr: context cancelled")
				return
			}
			if errors.Is(err, io.EOF) {
				slog.Info("dsmr: source exhausted")
				return
			}

			slog.Error("dsmr: failed to retrieve telegram", "error", err)
			select {
			case <-time.After(100 * time.Millisecond):
			case <-ctx.Done():
				return
			}
			continue
		}

		metrics, err := decodeTelegram(telegramRaw)
		if err != nil {
			slog.Error("dsmr: failed to parse telegram", "error", err)
			continue
		}

		handler.Handle(ctx, metrics)
	}
}

type source interface {
	Next(ctx context.Context) (string, error)
	Close() error
}

func newSource(cfg Config) (source, error) {
	switch cfg.Mode {
	case ModeSerial:
		return newSerialSource(cfg.Serial)
	case ModeMock:
		return newMockSource(cfg.Mock)
	default:
		return nil, fmt.Errorf("dsmr: unsupported mode %q", cfg.Mode)
	}
}

type serialSource struct {
	port   io.ReadCloser
	reader *bufio.Reader
}

func newSerialSource(cfg SerialConfig) (*serialSource, error) {
	if cfg.Device == "" {
		return nil, errors.New("dsmr: serial device is required")
	}

	// @TODO: later on build in a way to auto-detect the serial port. For now this is a good dev default.
	baud := cfg.BaudRate
	if baud == 0 {
		baud = 115200
	}

	timeout := cfg.ReadTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	serialCfg := &serial.Config{
		Name:        cfg.Device,
		Baud:        baud,
		ReadTimeout: timeout,
	}

	port, err := serial.OpenPort(serialCfg)
	if err != nil {
		return nil, fmt.Errorf("dsmr: open serial port: %w", err)
	}

	return &serialSource{
		port:   port,
		reader: bufio.NewReader(port),
	}, nil
}

func (s *serialSource) Next(ctx context.Context) (string, error) {
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		peek, err := s.reader.Peek(1)
		if err != nil {
			if errors.Is(err, io.EOF) {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return "", err
		}

		if len(peek) == 0 || peek[0] != '/' {
			if _, err := s.reader.ReadByte(); err != nil {
				if errors.Is(err, io.EOF) {
					time.Sleep(50 * time.Millisecond)
					continue
				}
				return "", err
			}
			continue
		}

		frame, err := s.reader.ReadBytes('!')
		if err != nil {
			if errors.Is(err, io.EOF) {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			return "", err
		}

		crcLine, err := s.reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			return "", err
		}

		return string(frame) + crcLine, nil
	}
}

func (s *serialSource) Close() error {
	if s.port == nil {
		return nil
	}
	return s.port.Close()
}

type mockSource struct {
	telegrams []string
	interval  time.Duration
	repeat    bool
	cursor    int
	mu        sync.Mutex
}

func newMockSource(cfg MockConfig) (*mockSource, error) {
	telegramList := cfg.Telegrams
	if len(telegramList) == 0 {
		telegramList = []string{defaultSampleTelegram}
	}

	var cleaned []string
	for _, t := range telegramList {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if !strings.HasSuffix(t, "\n") {
			t += "\n"
		}
		cleaned = append(cleaned, t)
	}

	if len(cleaned) == 0 {
		return nil, errors.New("dsmr: no mock telegrams provided")
	}

	interval := cfg.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}

	return &mockSource{
		telegrams: cleaned,
		interval:  interval,
		repeat:    cfg.Repeat,
	}, nil
}

// I need to add some more...
const defaultSampleTelegram = "/ISk5\x02MT382-1000\r\n0-0:1.0.0(230623150405S)\r\n1-0:1.7.0(00123.456*kW)\r\n1-0:2.7.0(00000.000*kW)\r\n!0000\r\n"

func (m *mockSource) Next(ctx context.Context) (string, error) {
	timer := time.NewTimer(m.interval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-timer.C:
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cursor >= len(m.telegrams) {
		if !m.repeat {
			return "", io.EOF
		}
		m.cursor = 0
	}

	tele := m.telegrams[m.cursor]
	m.cursor++
	return tele, nil
}

func (m *mockSource) Close() error { return nil }

func decodeTelegram(raw string) (SmartMeterData, error) {
	tele, err := dsmr.ParseTelegram(raw)
	if err != nil {
		return SmartMeterData{}, err
	}

	data := SmartMeterData{Timestamp: time.Now().Format(time.RFC3339)}

	set := func(raw string, assign func(float64)) {
		value, err := parseMeasurement(raw)
		if err != nil {
			slog.Warn("dsmr: skipping value", "raw", raw, "error", err)
			return
		}
		assign(value)
	}

	if raw, ok := tele.InstantaneousVoltageL1(); ok {
		set(raw, func(v float64) { data.InstVoltL1 = v })
	}
	if raw, ok := tele.InstantaneousVoltageL2(); ok {
		set(raw, func(v float64) { data.InstVoltL2 = v })
	}
	if raw, ok := tele.InstantaneousVoltageL3(); ok {
		set(raw, func(v float64) { data.InstVoltL3 = v })
	}
	if raw, ok := tele.InstantaneousCurrentL1(); ok {
		set(raw, func(v float64) { data.InstCurrentL1 = v })
	}
	if raw, ok := tele.InstantaneousCurrentL2(); ok {
		set(raw, func(v float64) { data.InstCurrentL2 = v })
	}
	if raw, ok := tele.InstantaneousCurrentL3(); ok {
		set(raw, func(v float64) { data.InstCurrentL3 = v })
	}
	if raw, ok := tele.MeterReadingElectricityDeliveredToClientTariff1(); ok {
		set(raw, func(v float64) { data.PowerDeliveredTariff1 = v })
	}
	if raw, ok := tele.MeterReadingElectricityDeliveredToClientTariff2(); ok {
		set(raw, func(v float64) { data.PowerDeliveredTariff2 = v })
	}
	if raw, ok := tele.MeterReadingElectricityDeliveredByClientTariff1(); ok {
		set(raw, func(v float64) { data.PowerGeneratedTariff1 = v })
	}
	if raw, ok := tele.MeterReadingElectricityDeliveredByClientTariff2(); ok {
		set(raw, func(v float64) { data.PowerGeneratedTariff2 = v })
	}
	if raw, ok := tele.MeterReadingGasDeliveredToClient(1); ok {
		set(raw, func(v float64) { data.GasDelivered = v })
	}
	if raw, ok := tele.ActualElectricityPowerDelivered(); ok {
		set(raw, func(v float64) { data.CurrentPowerConsumption = v })
	}
	if raw, ok := tele.ActualElectricityPowerReceived(); ok {
		set(raw, func(v float64) { data.CurrentPowerGeneration = v })
	}

	return data, nil
}

func parseMeasurement(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "*"); i >= 0 {
		raw = raw[:i]
	}
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseFloat(raw, 64)
}

func ConfigFromEnv() Config {
	mode := Mode(strings.ToLower(strings.TrimSpace(os.Getenv("DSMR_MODE"))))
	if mode == "" {
		mode = ModeMock
	}
	if mode != ModeSerial && mode != ModeMock {
		slog.Warn("dsmr: unknown mode, falling back to mock", "mode", mode)
		mode = ModeMock
	}

	serialCfg := SerialConfig{
		Device:      strings.TrimSpace(os.Getenv("DSMR_SERIAL_DEVICE")),
		BaudRate:    intFromEnv("DSMR_SERIAL_BAUD", 115200),
		ReadTimeout: durationFromEnv("DSMR_SERIAL_TIMEOUT", 2*time.Second),
	}

	mockCfg := MockConfig{
		Interval:  durationFromEnv("DSMR_MOCK_INTERVAL", 2*time.Second),
		Repeat:    boolFromEnv("DSMR_MOCK_REPEAT", true),
		Telegrams: mockTelegramsFromEnv(),
	}

	return Config{
		Mode:   mode,
		Serial: serialCfg,
		Mock:   mockCfg,
	}
}

func intFromEnv(key string, def int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		slog.Warn("dsmr: invalid int env", "key", key, "value", raw, "error", err)
		return def
	}
	return value
}

func durationFromEnv(key string, def time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("dsmr: invalid duration env", "key", key, "value", raw, "error", err)
		return def
	}
	return value
}

func boolFromEnv(key string, def bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		slog.Warn("dsmr: invalid bool env", "key", key, "value", raw, "error", err)
		return def
	}
	return value
}

// This can be cleaned up later...
func mockTelegramsFromEnv() []string {
	if path := strings.TrimSpace(os.Getenv("DSMR_MOCK_TELEGRAM_FILE")); path != "" {
		bytes, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("dsmr: failed to read mock telegram file", "path", path, "error", err)
		} else {
			if telegrams := splitTelegrams(string(bytes)); len(telegrams) > 0 {
				return telegrams
			}
		}
	}

	raw := os.Getenv("DSMR_MOCK_TELEGRAM")
	return splitTelegrams(raw)
}

func splitTelegrams(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	separators := []string{"\n\n", "\r\n\r\n", "---"}
	for _, sep := range separators {
		if strings.Contains(raw, sep) {
			parts := strings.Split(raw, sep)
			var result []string
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					result = append(result, part)
				}
			}
			return result
		}
	}

	return []string{raw}
}
