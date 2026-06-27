package tts

import (
	"context"
	"database/sql"
	"fmt"
	"goirc/db/model"
	db "goirc/model"
	"goirc/internal/responder"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func Handle(params responder.Responder) error {
	text := params.Match(1)
	if text == "" {
		return nil
	}

	return Speak(text, params)
}

func Speak(text string, params responder.Responder) error {
	if len(text) > 420 {
		text = text[:420]
	}

	ttsURL := fmt.Sprintf(
		"https://translate.google.com/translate_tts?ie=UTF-8&q=%s&tl=en&client=tw-ob",
		url.QueryEscape(text),
	)

	resp, err := http.Get(ttsURL)
	if err != nil {
		params.Privmsgf(params.Target(), "tts error: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		params.Privmsgf(params.Target(), "tts error: status %d", resp.StatusCode)
		return nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		params.Privmsgf(params.Target(), "tts error: %v", err)
		return err
	}

	if len(data) == 0 || strings.HasPrefix(http.DetectContentType(data), "text/html") {
		params.Privmsgf(params.Target(), "tts: no audio returned")
		return nil
	}

	q := model.New(db.DB.DB)
	file, err := q.InsertFile(context.Background(), model.InsertFileParams{
		Nick:    params.Nick(),
		Content: data,
	})
	if err != nil {
		params.Privmsgf(params.Target(), "tts error: %v", err)
		return err
	}

	_ = q.UpdateFileMime(context.Background(), model.UpdateFileMimeParams{
		ID:   file.ID,
		Mime: sql.NullString{String: "audio/mpeg", Valid: true},
	})

	url := fmt.Sprintf("%s/uploads/%d", os.Getenv("ROOT_URL"), file.ID)
	params.Privmsgf(params.Target(), "%s: %s", params.Nick(), url)

	return nil
}
