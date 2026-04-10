package main

import (
	"github.com/itkln/github-subscription/internal/app"
)

func main() {
	if err := app.Start(); err != nil {
		panic(err)
	}
}
