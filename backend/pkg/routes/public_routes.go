package routes

import (
	"github.com/erwindouna/p1ngry/app/controllers"
	"github.com/gofiber/fiber/v2"
)

// PublicRoutes func for describe group of public routes.
func PublicRoutes(a *fiber.App) {
	// Create routes group.
	route := a.Group("/api/v1")

	// Routes for GET method:
	route.Get("/books", controllers.GetBooks)
	route.Get("/book/:id", controllers.GetBook)

	// Routes for POST method:
	route.Post("/user/sign/up", controllers.UserSignUp)
	route.Post("/user/sign/in", controllers.UserSignIn)

	route.Get("/dsmr/sse", controllers.SSESerializeReading)
}
