package dsmr

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type MQTTConfig struct {
	Broker    string
	ClientID  string
	Username  string
	Password  string
	Topic     string
	Connected bool
}

const defaultMQTTBuffer = 16

type MQTTPublisher struct {
	cfg    MQTTConfig
	client mqtt.Client
	topic  string

	ctx    context.Context
	cancel context.CancelFunc

	buffer chan SmartMeterData
	dedupe bool

	mu   sync.RWMutex
	last *SmartMeterData
}

func NewMQTTClient(cfg MQTTConfig) mqtt.Client {
	if cfg.Broker == "" {
		slog.Error("mqtt: broker not configured; skipping client startup")
		return nil
	}

	slog.Info("mqtt: initializing new client", "broker", cfg.Broker, "clientID", cfg.ClientID)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker)
	opts.SetClientID(cfg.ClientID)
	opts.SetUsername(cfg.Username)
	opts.SetPassword(cfg.Password)

	opts.AutoReconnect = true
	opts.ConnectRetry = true
	opts.ConnectRetryInterval = 2 * time.Second

	opts.OnConnect = func(c mqtt.Client) {
		slog.Info("mqtt: connected")
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		slog.Error("mqtt: connection lost", "error", err)
	}

	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		slog.Error("mqtt: connection error", "error", err)
	}

	return client
}

func NewMQTTPublisher(cfg MQTTConfig) *MQTTPublisher {
	if cfg.Broker == "" || cfg.Topic == "" {
		slog.Info("mqtt: publisher disabled", "broker", cfg.Broker, "topic", cfg.Topic)
		return nil
	}

	client := NewMQTTClient(cfg)
	if client == nil {
		slog.Warn("mqtt: client initialisation failed; publisher disabled")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	pub := &MQTTPublisher{
		cfg:    cfg,
		client: client,
		topic:  cfg.Topic,
		ctx:    ctx,
		cancel: cancel,
		buffer: make(chan SmartMeterData, defaultMQTTBuffer),
		dedupe: true,
	}

	RegisterHandler(HandlerFunc(pub.handleTelegram))
	go pub.run()
	return pub
}

func NewMQTTPublisherFromEnv() *MQTTPublisher {
	return NewMQTTPublisher(MQTTConfigFromEnv())
}

func (p *MQTTPublisher) handleTelegram(ctx context.Context, data SmartMeterData) {
	if p == nil {
		return
	}
	select {
	case p.buffer <- data:
	case <-ctx.Done():
	case <-p.ctx.Done():
	default:
		slog.Warn("mqtt: dropping telegram; publisher busy")
	}
}

func (p *MQTTPublisher) run() {
	for {
		select {
		case <-p.ctx.Done():
			slog.Info("mqtt: publisher stopped")
			return
		case data := <-p.buffer:
			snapshot := data
			if p.dedupe {
				snapshot.Timestamp = ""
				if p.isDuplicate(snapshot) {
					slog.Info("mqtt: skipping duplicate telegram")
					continue
				}
			}

			if err := PublishSmartMeterData(p.client, p.topic, data); err != nil {
				slog.Error("mqtt: publish failed", "error", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			if p.dedupe {
				p.markPublished(snapshot)
			}
		}
	}
}

func (p *MQTTPublisher) isDuplicate(snapshot SmartMeterData) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.last == nil {
		return false
	}
	// I don't even have to hash it. Lovely function!
	return reflect.DeepEqual(*p.last, snapshot)
}

func (p *MQTTPublisher) markPublished(snapshot SmartMeterData) {
	p.mu.Lock()
	defer p.mu.Unlock()
	copy := snapshot
	p.last = &copy
}

func (p *MQTTPublisher) Close() {
	if p == nil {
		return
	}
	p.cancel()
	if p.client != nil && p.client.IsConnectionOpen() {
		slog.Info("mqtt: disconnecting")
		p.client.Disconnect(250)
	}
}

func PublishSmartMeterData(client mqtt.Client, topic string, data SmartMeterData) error {
	if client == nil {
		slog.Error("mqtt: publish skipped; client not initialised")
		return nil
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	token := client.Publish(topic, 0, false, payload)
	token.Wait()
	return token.Error()
}

func MQTTConfigFromEnv() MQTTConfig {
	broker := strings.TrimSpace(os.Getenv("MQTT_BROKER"))
	clientID := strings.TrimSpace(os.Getenv("MQTT_CLIENT_ID"))
	username := strings.TrimSpace(os.Getenv("MQTT_USERNAME"))
	password := strings.TrimSpace(os.Getenv("MQTT_PASSWORD"))
	topic := strings.TrimSpace(os.Getenv("MQTT_TOPIC"))

	return MQTTConfig{
		Broker:   broker,
		ClientID: clientID,
		Username: username,
		Password: password,
		Topic:    topic,
	}
}
