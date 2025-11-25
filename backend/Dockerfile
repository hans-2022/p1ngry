FROM golang:1.25-alpine AS builder

LABEL maintainer="Erwin Douna <e.douna@gmail.com> (https://github.com/erwindouna)"

# Move to working directory (/build).
WORKDIR /build

# Copy and download dependency using go mod.
COPY go.mod go.sum ./
RUN go mod download

# Copy the code into the container.
COPY . .

# Set necessary environment variables needed for our image and build the API server.
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -ldflags="-s -w" -o p1ngry .

FROM scratch

# Copy binary and config files from /build to root folder of scratch container.
COPY --from=builder ["/build/p1ngry", "/build/.env", "/"]

# Command to run when starting the container.
ENTRYPOINT ["/p1ngry"]