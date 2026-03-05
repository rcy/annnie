package summary

import (
	"bytes"
	"context"
	_ "embed"
	"goirc/db/model"
	"goirc/internal/ai"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type link struct {
	Title string
	URL   string
}

type completeFn func(context.Context, string, string) (string, error)
type getTitleFn func(string) (string, error)

type summary struct {
	queries       *model.Queries
	completeFn    completeFn
	getTitleFn    getTitleFn
	Start         time.Time
	End           time.Time
	Notes         map[string][]model.Note
	NotesSummary  string
	QuotesSummary string
	Links         []link
}

func New(queries *model.Queries, start time.Time, end time.Time) *summary {
	return &summary{
		queries:    queries,
		completeFn: ai.Complete,
		getTitleFn: getTitle,
		Start:      start,
		End:        end,
	}
}

func (s *summary) LoadNotes(ctx context.Context) error {
	notes, err := s.queries.NotesBetween(ctx, model.NotesBetweenParams{
		StartAt: s.Start,
		EndAt:   s.End,
	})
	if err != nil {
		return err
	}

	s.Notes = make(map[string][]model.Note)
	for _, note := range notes {
		if note.Target == note.Nick.String {
			// undelivered anonymous note
			continue
		}
		key := note.Kind
		s.Notes[key] = append(s.Notes[key], note)
	}

	return nil
}

func (s *summary) SummarizeNotes(ctx context.Context) error {
	texts := make([]string, len(s.Notes["note"]))
	for i, note := range s.Notes["note"] {
		texts[i] = note.Nick.String + " said " + note.Text.String
	}

	if len(texts) > 0 {
		completion, err := s.completeFn(ctx, "you will be provide a selection of statements shared to an irc channel over. connect these statements as a brief and engaging summary",
			strings.Join(texts, "\n"))
		if err != nil {
			return err
		}
		s.NotesSummary = completion
	}
	return nil
}

func (s *summary) SummarizeQuotes(ctx context.Context) error {
	texts := make([]string, len(s.Notes["quote"]))
	for i, note := range s.Notes["quote"] {
		texts[i] = strings.TrimPrefix(note.Text.String, "\"")
	}

	if len(texts) > 0 {
		completion, err := s.completeFn(ctx, "you will be provide a selection of headlines. create an engaging summary of them.  omit any preamble and don't over editorialize.",
			strings.Join(texts, "\n"))
		if err != nil {
			return err
		}
		s.QuotesSummary = completion
	}

	return nil
}

func (s *summary) ProcessLinks(ctx context.Context) error {
	s.Links = make([]link, len(s.Notes["link"]))

	for i, link := range s.Notes["link"] {
		s.Links[i].URL = link.Text.String

		title, err := s.getTitleFn(link.Text.String)
		if err != nil {
			s.Links[i].Title = err.Error()
		} else {
			s.Links[i].Title = title
		}
	}
	return nil
}

func (s *summary) LoadAll(ctx context.Context) error {
	err := s.LoadNotes(ctx)
	if err != nil {
		return err
	}

	err = s.SummarizeNotes(ctx)
	if err != nil {
		return err
	}

	err = s.SummarizeQuotes(ctx)
	if err != nil {
		return err
	}

	err = s.ProcessLinks(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *summary) SetGetTitleFn(fn getTitleFn) {
	s.getTitleFn = fn
}
func (s *summary) SetCompleteFn(fn completeFn) {
	s.completeFn = fn
}

func getTitle(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	z := html.NewTokenizer(resp.Body)
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return "", nil
		case html.StartTagToken, html.SelfClosingTagToken:
			tName, _ := z.TagName()
			if string(tName) == "title" {
				z.Next()
				return strings.TrimSpace(string(z.Text())), nil
			}
		}
	}
}

var funcMap = template.FuncMap{
	"dateStart": func(t time.Time) string {
		return t.Format("Mon Jan 02")
	},
	"dateEnd": func(t time.Time) string {
		s := t.Add(-time.Second)
		return s.Format("Mon Jan 02, 2006")
	},
}

//go:embed news.html
var newsTemplateContent string
var newsTemplate = template.Must(template.New("").Funcs(funcMap).Parse(newsTemplateContent))

func (s *summary) HTML(ctx context.Context) ([]byte, error) {
	err := s.LoadNotes(ctx)
	if err != nil {
		return nil, err
	}

	err = s.SummarizeNotes(ctx)
	if err != nil {
		return nil, err
	}

	err = s.SummarizeQuotes(ctx)
	if err != nil {
		return nil, err
	}

	err = s.ProcessLinks(ctx)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = newsTemplate.Execute(&buf, s)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// return the sunday prior to the time passed in
func WeekStart(t time.Time, location *time.Location) time.Time {
	day := t.In(location)
	weekday := t.Weekday()
	offset := (int(weekday) - int(time.Sunday) + 7) % 7 // Days since last Sunday
	return day.AddDate(0, 0, -offset)
}

func (s *summary) WeeklyNewsletter(ctx context.Context) ([]byte, error) {
	err := s.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	b, err := s.HTML(ctx)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (s *summary) cacheKey() string {
	return s.Start.Format(time.DateOnly) + "_" + s.End.Format(time.DateOnly)
}

var cacheMap sync.Map

func (s *summary) MemoryCache(ctx context.Context, fn func(context.Context) ([]byte, error)) ([]byte, error) {
	cached, ok := cacheMap.Load(s.cacheKey())
	if ok {
		return cached.([]byte), nil
	}

	bytes, err := fn(ctx)
	if err != nil {
		return nil, err
	}
	cacheMap.Store(s.cacheKey(), bytes)
	return bytes, nil
}

func (s *summary) DBCache(ctx context.Context, q *model.Queries, fn func(context.Context) ([]byte, error)) ([]byte, error) {
	cached, err := q.CacheLoad(ctx, s.cacheKey())
	if err == nil {
		return []byte(cached.Value), nil
	}

	bytes, err := fn(ctx)
	if err != nil {
		return nil, err
	}
	_, err = q.CacheStore(ctx, model.CacheStoreParams{
		Key:   s.cacheKey(),
		Value: string(bytes),
	})
	if err != nil {
		return nil, err
	}
	return bytes, nil
}
