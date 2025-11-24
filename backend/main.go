package main

import (
	"context"
	"log/slog"
	"os"
	"sync"
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

// @title API
// @version 1.0
// @description This is an auto-generated API Docs.
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.email your@mail.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @BasePath /api
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
func main() {
	// Define Fiber config.
	slog.Info("Starting p1ngry")
	config := configs.FiberConfig()

	// Define a new Fiber app with config.
	app := fiber.New(config)

	// Middlewares.
	middleware.FiberMiddleware(app) // Register Fiber's middleware for app.

	// Routes.
	routes.SwaggerRoute(app)  // Register a route for API Docs (Swagger).
	routes.PublicRoutes(app)  // Register a public routes for app.
	routes.PrivateRoutes(app) // Register a private routes for app.
	routes.NotFoundRoute(app) // Register route for 404 Error.

	slog.Info("Starting P1 reader service")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	jobs := make(chan string, 100)
	if err := dsmr.StartP1Reader(ctx, jobs, &wg); err != nil {
		slog.Error("Failed to start P1 reader", "error", err)
		return
	}

	numWorkers := 5
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go dsmr.Worker(ctx, i, jobs, &wg)
	}

	time.AfterFunc(time.Minute*5, func() {
		slog.Info("Shutting down P1 reader service")
		cancel()
		wg.Wait()
	})

	// Start server (with or without graceful shutdown).
	if os.Getenv("STAGE_STATUS") == "dev" {
		utils.StartServer(app)
	} else {
		utils.StartServerWithGracefulShutdown(app)
	}
}
