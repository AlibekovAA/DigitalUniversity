package maxAPI

import (
	"context"
	"digitalUniversity/config"
	"digitalUniversity/logger"

	"github.com/jmoiron/sqlx"
	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

type Bot struct {
	MaxBot *schemes.BotInfo
	db     *sqlx.DB
	logger *logger.Logger
	MaxAPI *maxbot.Api
}

func NewMaxBot(t *Bot, config *config.MaxConfig, ctx context.Context) (*maxbot.Api, *schemes.BotInfo, error) {
	api, err := maxbot.New(config.Token)
	if err != nil {
		return nil, nil, err
	}

	b, err := api.Bots.GetBot(ctx)
	if err != nil {
		return nil, nil, err
	}

	return api, b, nil
}

func NewBot(config *config.MaxConfig, logger *logger.Logger, db *sqlx.DB, ctx context.Context) (*Bot, error) {
	b := &Bot{
		db:     db,
		logger: logger,
	}

	api, maxBot, err := NewMaxBot(b, config, ctx)
	if err != nil {
		b.logger.Errorf("failed create telegram bot %v", err)
		return nil, err
	}

	b.MaxBot = maxBot
	b.MaxAPI = api

	return b, nil
}

func (b *Bot) Start(ctx context.Context) {
	go func() {
		for upd := range b.MaxAPI.GetUpdates(ctx) {
			b.logger.Infof("Received update: %#v", upd)

			switch msg := upd.(type) {
			case *schemes.MessageCreatedUpdate:
				resp := maxbot.NewMessage().
					SetUser(msg.Message.Sender.UserId).
					SetText(msg.Message.Body.Text)

				_, err := b.MaxAPI.Messages.Send(ctx, resp)
				if err != nil {
					b.logger.Errorf("Failed to send echo message: %v", err)
				}
			default:
				b.logger.Debugf("Unhandled update type: %T", upd)
			}
		}
	}()

}
