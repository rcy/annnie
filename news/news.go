package news

import (
	"bytes"
	"context"
	"fmt"
	"goirc/fetch"
	"goirc/internal/ai"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func NewsByTopic(ctx context.Context, topic string, length int) (string, error) {
	headlines, err := cnnHeadlines()
	if err != nil {
		return "", err
	}

	completion, err := ai.Complete(ctx,
		fmt.Sprintf("Summarize the following headlines in %d characters or less.", length),
		fmt.Sprintf("Only consider headlines pertaining to the topic: %s.  If no headlines match the topic, expand the topic until you can report some news. \n\nHeadlines follow: %s\n",
			topic,
			strings.Join(headlines, ".\n")),
		false,
	)
	if err != nil {
		return "", err
	}
	return completion, nil
}

func News(ctx context.Context, length int) (string, error) {
	headlines, err := cnnHeadlines()
	if err != nil {
		return "", err
	}

	completion, err := ai.Complete(ctx,
		fmt.Sprintf("Summarize the following headlines in %d characters or less.", length),
		strings.Join(headlines, ".\n"),
		false)
	if err != nil {
		return "", err
	}
	return completion, nil
}

func cnnHeadlines() ([]string, error) {
	code, body, err := fetch.Get("https://lite.cnn.com", time.Minute)
	if err != nil {
		return nil, err
	}

	if code > 299 {
		return nil, fmt.Errorf("bad code: %d", code)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	headlines := []string{}

	doc.Find(".card--lite").Each(func(i int, s *goquery.Selection) {
		headlines = append(headlines, strings.TrimSpace(s.Text()))
	})

	return headlines, nil
}
