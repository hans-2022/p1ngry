package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/erwindouna/p1ngry/pkg/configs"
	"github.com/erwindouna/p1ngry/pkg/dsmr"
	"github.com/erwindouna/p1ngry/pkg/middleware"
	"github.com/erwindouna/p1ngry/pkg/routes"
	"github.com/erwindouna/p1ngry/pkg/utils"

	"github.com/gofiber/fiber/v2"

	_ "github.com/erwindouna/p1ngry/docs" // load API Docs files (Swagger)
	_ "github.com/joho/godotenv/autoload" // load .env file automatically
)

func main() {
	slog.Info("Starting p1ngry")

	config := configs.FiberConfig()

	// Define a new Fiber app with config.
	app := fiber.New(config)

	// Middlewares.
	middleware.FiberMiddleware(app)

	// Routes.
	routes.SwaggerRoute(app)
	routes.PublicRoutes(app)
	routes.PrivateRoutes(app)
	routes.NotFoundRoute(app)

	// Put it in a goroutine to not block the main thread.
	publisherCh := make(chan *dsmr.MQTTPublisher, 1)
	go func() {
		publisherCh <- dsmr.NewMQTTPublisherFromEnv()
	}()

	var publisher *dsmr.MQTTPublisher
	select {
	case publisher = <-publisherCh:
		if publisher != nil {
			defer publisher.Close()
		}
	case <-time.After(3 * time.Second):
		slog.Error("mqtt: startup timed out; continuing without publisher")
	}

	// Same for the DSMR reader, it also gets it own goroutine.
	// This one can be more simple, since it spings up in a goroutine in Start.
	go dsmr.RunReader()
	defer dsmr.Stop()

	// Start server (with or without graceful shutdown).
	if os.Getenv("STAGE_STATUS") == "dev" {
		slog.Warn("Running in development mode!")
		utils.StartServer(app)
	} else {
		utils.StartServerWithGracefulShutdown(app)
	}
}
