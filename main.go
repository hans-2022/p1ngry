package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

func main() {
	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("p1ngry is running!")
	})

	log.Println("Listening on :3000")
	log.Fatal(app.Listen(":3000"))
}
