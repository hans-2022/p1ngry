FROM alpine:3.20
# new container apprach

RUN apk add --no-cache jq

LABEL \
  io.hass.version="0.0.5-HD4.2" \
  io.hass.type="app" \
  io.hass.arch="aarch64|amd64"

WORKDIR /app

COPY p1nrgy /usr/local/bin/p1nrgy
COPY entrypoint.sh /usr/local/bin/entrypoint.sh

RUN chmod +x /usr/local/bin/p1nrgy /usr/local/bin/entrypoint.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
