package news

import (
	"context"
	"fmt"
	"goirc/internal/responder"
	"goirc/news"
	"log/slog"
)

func Handle(params responder.Responder) error {
	slog.Info("reading the news", "params", params)
	var topic string
	if len(params.Matches()) > 1 {
		topic = params.Match(1)
	}

	ctx := context.TODO()

	var result string
	var err error

	if topic == "" {
		result, err = news.News(ctx, 420)
		if err != nil {
			return fmt.Errorf("News: %w", err)
		}
	} else {
		result, err = news.NewsByTopic(ctx, topic, 420)
		if err != nil {
			return fmt.Errorf("NewsByTopic: %w", err)
		}
	}

	params.Privmsgf(params.Target(), "%s", result)
	return nil
}
