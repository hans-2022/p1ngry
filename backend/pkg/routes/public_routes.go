package routes

import (
	"github.com/erwindouna/p1ngry/app/controllers"
	"github.com/gofiber/fiber/v2"
)

// PublicRoutes func for describe group of public routes.
func PublicRoutes(a *fiber.App) {
	// Create routes group.
	route := a.Group("/api/v1")

	route.Get("/dsmr/sse", controllers.SSESerializeReading)
}
