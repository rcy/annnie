package tz

import (
	"context"
	"fmt"
	"goirc/config"
	"goirc/db/model"
	"goirc/internal/responder"
	db "goirc/model"
)

func Handle(params responder.Responder) error {
	tz, err := getNickTimezone(context.TODO(), params.Nick())
	if err != nil {
		params.Privmsgf(params.Target(), "%s: I don't know your timezone. Visit %s to set it", params.Nick(), config.Get().RootURL)
		return nil
	}

	params.Privmsgf(params.Target(), "%s: your timezone is %s", params.Nick(), tz)

	return nil
}

func getNickTimezone(ctx context.Context, nick string) (string, error) {
	q := model.New(db.DB.DB)
	nickTimezone, err := q.GetNickTimezone(ctx, nick)
	if err != nil {
		return "", fmt.Errorf("GetNickTimezone: %w", err)
	}
	return nickTimezone.Tz, nil
}
