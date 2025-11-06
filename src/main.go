package main

import (
	"context"
	"digitalUniversity/application"
	"digitalUniversity/config"
	"log"
	"os"
	"os/signal"

	_ "github.com/lib/pq"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed load config %+v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)

	defer cancel()

	app := application.NewApplication()

	app.Configure(cfg, ctx)

	app.Run(ctx)
}
