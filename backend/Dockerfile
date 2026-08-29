FROM alpine:3.20

LABEL \
  io.hass.version="0.0.5-HD.2" \
  io.hass.type="app" \
  io.hass.arch="aarch64|amd64"

WORKDIR /app
COPY p1nrgy /usr/local/bin/p1nrgy
ENTRYPOINT ["/usr/local/bin/p1nrgy"]
