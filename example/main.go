package main

import (
	"log"

	"github.com/lumm2509/keel"
)

type Cradle struct {
	Name string
}

func main() {
	app := keel.New(keel.WithCradle(Cradle{Name: "example"}))

	app.GET("/hello", func(c *keel.Context[Cradle]) error {
		return c.JSON(200, map[string]string{
			"hello": c.Cradle().Name,
		})
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
