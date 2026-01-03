package day

import (
	"context"
	"fmt"
	"goirc/image"
	"goirc/internal/ai"
	"goirc/internal/responder"
	"goirc/shell"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
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

	completion, err := ai.Complete(context.TODO(), openai.GPT4Dot1, "in one short sentence, imperatively and cynically describe a way to celebrate the given national day to your friends in the chat.  be terse use dry humour and minimal punctuation.", event)
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

func fetchDayEvents(day string) (*stack, error) {
	// daysoftheyear website uses 3 day months, except for september:
	// jan,feb,mar,apr,may,jun,jul,aug,sept(!),oct,nov,dec
	// the format of day argument is mmm/dd, so handle sep/sept special case here
	day = strings.Replace(day, "sep/", "sept/", 1)

	url := fmt.Sprintf("https://www.daysoftheyear.com/days/%s", day)
	cmd := fmt.Sprintf(`curl --location -s %s | pup 'body img json{}' | jq -r .[].alt | grep -E '\bDay\b'`, url)
	r, err := shell.Command(cmd)
	if err != nil {
		return nil, err
	}
	r = strings.TrimSpace(r)
	events := strings.Split(r, "\n")

	return &stack{items: events}, nil
}

// TODO: this shouldn't be here
func Image(params responder.Responder) error {
	prompt := params.Match(1)
	gi, err := image.GenerateDALLE(context.Background(), prompt)
	if err != nil {
		return err
	}

	params.Privmsgf(params.Target(), "%s", gi.URL())

	return nil
}
