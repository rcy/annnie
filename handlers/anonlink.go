package handlers

import (
	"context"
	"goirc/db/model"
	"goirc/handlers/linkpool"
	"goirc/internal/responder"
	db "goirc/model"
	"time"
)

const (
	// how old a message must be before it is delivered
	minAge = 7 * time.Hour * 24
)

func AnonLink(params responder.Responder) error {
	q := model.New(db.DB)
	pool := linkpool.New(q, minAge)
	note, err := pool.PopRandomNote(context.Background(), params.Target(), "link")
	if err != nil {
		return err
	}

	text, err := note.Link()
	if err != nil {
		return err
	}

	params.Privmsgf(params.Target(), "%s", text)
	return nil
}

func AnonQuote(params responder.Responder) error {
	q := model.New(db.DB)
	pool := linkpool.New(q, minAge)
	note, err := pool.PopRandomNote(context.Background(), params.Target(), "quote")
	if err != nil {
		return err
	}

	params.Privmsgf(params.Target(), "%s", note.Text.String)

	return nil
}

func AnonStatus(params responder.Responder) error {
	ctx := context.TODO()
	q := model.New(db.DB)
	allPool := linkpool.New(q, 0)
	allLinks, err := allPool.Notes(ctx, "link")
	if err != nil {
		return err
	}
	allQuotes, err := allPool.Notes(ctx, "quote")
	if err != nil {
		return err
	}

	dayPool := linkpool.New(q, minAge)
	dayLinks, err := dayPool.Notes(ctx, "link")
	if err != nil {
		return err
	}
	dayQuotes, err := dayPool.Notes(ctx, "quote")
	if err != nil {
		return err
	}

	params.Privmsgf(params.Target(), "links=%d+%d quotes=%d+%d",
		len(dayLinks), len(allLinks)-len(dayLinks),
		len(dayQuotes), len(allQuotes)-len(dayQuotes),
	)

	return nil
}
