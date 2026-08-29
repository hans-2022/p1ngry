FROM alpine:3.20
# new contaimer apprach
LABEL \
  io.hass.version="0.0.5-HD.3" \
  io.hass.type="app" \
  io.hass.arch="aarch64|amd64"

WORKDIR /app
COPY p1nrgy /usr/local/bin/p1nrgy
ENTRYPOINT ["/usr/local/bin/p1nrgy"]
