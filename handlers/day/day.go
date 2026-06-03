package day

import (
	"errors"
	"fmt"
	"goirc/handlers/annie"
	"goirc/image"
	"goirc/internal/ai"
	"goirc/internal/responder"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type stack struct {
	items []string
}

func (s *stack) Pop() (string, bool) {
	if len(s.items) == 0 {
		return "", false
	}
	item := s.items[0]
	s.items = s.items[1:]
	return item, true
}

var dayDays = make(map[string]*stack)

func NationalDay(params responder.Responder) error {
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return err
	}
	today := strings.ToLower(time.Now().In(location).Format("Jan/02"))

	event, err := getEvent(today)
	if err != nil {
		return err
	}
	if event == "" {
		params.Privmsgf(params.Target(), "thats it")
		return nil
	}

	override, err := annie.GetSystemOverride(params.Context())
	if err != nil {
		return fmt.Errorf("getSystemOverride: %w", err)
	}

	completion, err := ai.Complete(params.Context(), fmt.Sprintf("%s. in one short sentence, imperatively and cynically describe a way to celebrate the given national day to your friends in the chat.  be terse use dry humour and minimal punctuation.", override), event)
	if err != nil {
		return err
	}

	params.Privmsgf(params.Target(), "%s: %s", event, completion)
	return nil
}

func getEvent(day string) (string, error) {
	var err error
	stack, ok := dayDays[day]
	if !ok {
		stack, err = fetchDayEvents(day)
		if err != nil {
			return "", err
		}
		dayDays[day] = stack
	}

	event, ok := stack.Pop()
	if !ok {
		return "", nil
	}
	return event, nil
}

var fetchURL = func(day string) string {
	return fmt.Sprintf("https://www.daysoftheyear.com/days/%s", day)
}

func fetchDayEvents(day string) (*stack, error) {
	// daysoftheyear website uses 3 day months, except for september:
	// jan,feb,mar,apr,may,jun,jul,aug,sept(!),oct,nov,dec
	// the format of day argument is mmm/dd, so handle sep/sept special case here
	day = strings.Replace(day, "sep/", "sept/", 1)

	url := fetchURL(day)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.114 Safari/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var events []string
	doc.Find(".card__title a.js-link-target, article a.js-link-target").Each(func(_ int, s *goquery.Selection) {
		if text := strings.TrimSpace(s.Text()); dayWordRe.MatchString(text) {
			events = append(events, text)
		}
	})

	return &stack{items: events}, nil
}

var dayWordRe = regexp.MustCompile(`(?i)\bday\b`)

// TODO: this shouldn't be here
func Image(params responder.Responder) error {
	prompt := params.Match(1)
	start := time.Now()
	gi, err := image.GenerateGPTImage(params.Context(), prompt)
	if err != nil {
		if errors.Is(err, ai.ErrRejected) {
			elapsed := time.Since(start)
			if elapsed > 30*time.Second {
				return fmt.Errorf("Aborted after %s", elapsed)
			}
			return fmt.Errorf("Rejected in %s", elapsed)
		}
		return err
	}

	params.Privmsgf(params.Target(), "%s", gi.URL())

	return nil
}
