package application

import (
	"context"
	"digitalUniversity/config"
	"digitalUniversity/database"
	"digitalUniversity/maxAPI"
	"log"

	"github.com/jmoiron/sqlx"
)

type Application struct {
	Bot *maxAPI.Bot
	DB  *sqlx.DB
}

func NewApplication() *Application {
	return &Application{}
}

func (app *Application) Configure(cfg *config.Config, ctx context.Context) error {
	db, err := database.OpenDB(&cfg.Database)
	if err != nil {

		log.Printf("failed open DB %v", err)
		return err
	}

	app.DB = db

	// bot, err := telegram.NewBot(&cfg.Telegram, app.poller, cfg.Standup.ExcludeList, app.Conversation, db)
	// if err != nil {

	// 	log.Printf("failed create telegram bot %v", err)
	// 	return err
	// }

	//app.Bot = bot

	return nil
}

func (app *Application) Run(ctx context.Context) {
	//go
	//app.Bot.Start(ctx)

	app.DB.Close()

}
