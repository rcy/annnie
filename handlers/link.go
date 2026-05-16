package handlers

import (
	"context"
	"database/sql"
	"errors"
	"goirc/db/model"
	"goirc/internal/og"
	"goirc/internal/responder"
	db "goirc/model"
)

func Link(params responder.Responder) error {
	q := model.New(db.DB)

	url := params.Match(1)

	// posted in a private message?
	isAnonymous := params.Target() == params.Nick()

	tags, err := og.Fetch(context.TODO(), url)
	if err != nil {
		if !errors.Is(err, og.ErrNoTags) {
			return err
		}
	}

	_, err = q.InsertNote(context.TODO(), model.InsertNoteParams{
		Target:        params.Target(),
		Nick:          sql.NullString{String: params.Nick(), Valid: true},
		Kind:          "link",
		Text:          sql.NullString{String: url, Valid: true},
		Anon:          isAnonymous,
		OgTitle:       tags.Title,
		OgDescription: tags.Description,
		OgImage:       tags.Image,
	})
	if err != nil {
		return err
	}

	if isAnonymous {
		_, err = q.ScheduleFutureMessage(context.TODO(), "link")
		if err != nil {
			return err
		}

		params.Privmsgf(params.Target(), "thanks for the link")

		return nil
	}

	return nil
}
