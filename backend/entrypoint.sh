#!/bin/sh
set -eu

CONFIG="/data/options.json"

export DSMR_ENABLED="$(jq -r '.dsmr_enabled' "$CONFIG")"
export DSMR_MODE="$(jq -r '.dsmr_mode' "$CONFIG")"
export DSMR_SERIAL_DEVICE="$(jq -r '.dsmr_serial_device' "$CONFIG")"
export DSMR_SERIAL_BAUD="$(jq -r '.dsmr_serial_baud' "$CONFIG")"
export DSMR_SERIAL_DATABITS="$(jq -r '.dsmr_serial_databits' "$CONFIG")"
export DSMR_SERIAL_PARITY="$(jq -r '.dsmr_serial_parity' "$CONFIG")"
export DSMR_SERIAL_STOPBITS="$(jq -r '.dsmr_serial_stopbits' "$CONFIG")"

export DEBUG_LOGGING="$(jq -r '.debug_logging' "$CONFIG")"

MQTT_HOST="$(jq -r '.mqtt_host' "$CONFIG")"
MQTT_PORT="$(jq -r '.mqtt_port' "$CONFIG")"

export MQTT_BROKER="tcp://${MQTT_HOST}:${MQTT_PORT}"
export MQTT_USERNAME="$(jq -r '.mqtt_username' "$CONFIG")"
export MQTT_PASSWORD="$(jq -r '.mqtt_password' "$CONFIG")"
export MQTT_CLIENT_ID="$(jq -r '.mqtt_client_id' "$CONFIG")"
export MQTT_TOPIC="$(jq -r '.mqtt_topic' "$CONFIG")"

echo "p1nrgy entrypoint configuration:"
echo "  DSMR_ENABLED=${DSMR_ENABLED}"
echo "  DSMR_MODE=${DSMR_MODE}"
echo "  DSMR_SERIAL_DEVICE=${DSMR_SERIAL_DEVICE}"
echo "  DSMR_SERIAL_BAUD=${DSMR_SERIAL_BAUD}"
echo "  MQTT_BROKER=${MQTT_BROKER}"
echo "  MQTT_CLIENT_ID=${MQTT_CLIENT_ID}"
echo "  MQTT_TOPIC=${MQTT_TOPIC}"
echo "  DEBUG_LOGGING=${DEBUG_LOGGING}"

exec /usr/local/bin/p1nrgy
