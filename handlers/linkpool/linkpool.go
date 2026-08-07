package linkpool

import (
	"context"
	"database/sql"
	"errors"
	"goirc/db/model"
	"math/rand"
	"time"
)

type pool struct {
	rnd     *rand.Rand
	minAge  time.Duration
	queries queries
}

type queries interface {
	UnsentAnonymousNotes(context.Context, model.UnsentAnonymousNotesParams) ([]model.Note, error)
	UnsentAnonymousNotesAny(context.Context, time.Time) ([]model.Note, error)
	MarkAnonymousNoteDelivered(context.Context, model.MarkAnonymousNoteDeliveredParams) (model.Note, error)
	InsertNote(context.Context, model.InsertNoteParams) (model.Note, error)
}

func New(queries queries, minAge time.Duration) pool {
	return pool{
		queries: queries,
		rnd:     rand.New(rand.NewSource(time.Now().UnixNano())),
		minAge:  minAge,
	}
}

func (p pool) Seed(seed int64) {
	p.rnd.Seed(seed)
}

func (p pool) Notes(ctx context.Context, kind string) ([]model.Note, error) {
	olderThan := time.Now().UTC().Add(-p.minAge)
	notes, err := p.queries.UnsentAnonymousNotes(ctx, model.UnsentAnonymousNotesParams{
		CreatedAt: olderThan,
		Kind:      kind,
	})
	if err != nil {
		return []model.Note{}, err
	}
	return notes, nil
}

var NoNoteFoundError = errors.New("no note found")

func (p pool) PeekRandomNote(ctx context.Context, kind string) (model.Note, error) {
	notes, err := p.Notes(ctx, kind)
	if err != nil {
		return model.Note{}, err
	}
	if len(notes) == 0 {
		return model.Note{}, NoNoteFoundError
	}
	r := p.rnd.Intn(len(notes))
	return notes[r], nil
}

func (p pool) PeekRandomNoteAny(ctx context.Context) (model.Note, error) {
	olderThan := time.Now().UTC().Add(-p.minAge)
	notes, err := p.queries.UnsentAnonymousNotesAny(ctx, olderThan)
	if err != nil {
		return model.Note{}, err
	}
	if len(notes) == 0 {
		return model.Note{}, NoNoteFoundError
	}
	r := p.rnd.Intn(len(notes))
	return notes[r], nil
}

func (p pool) PopRandomNote(ctx context.Context, target string, kind string) (model.Note, error) {
	var note model.Note
	var err error
	if kind == "" {
		note, err = p.PeekRandomNoteAny(ctx)
		if err != nil {
			return model.Note{}, err
		}
	} else {
		note, err = p.PeekRandomNote(ctx, kind)
		if err != nil {
			return model.Note{}, err
		}
	}
	note, err = p.queries.MarkAnonymousNoteDelivered(ctx, model.MarkAnonymousNoteDeliveredParams{
		ID:     note.ID,
		Target: target,
	})
	if err != nil {
		return model.Note{}, err
	}
	return note, nil
}

type PushNoteParams struct {
	Target string
	Nick   string
	Kind   string
	Text   string
}

func (p pool) PushNote(ctx context.Context, params PushNoteParams) (model.Note, error) {
	return p.queries.InsertNote(ctx, model.InsertNoteParams{
		Target: params.Target,
		Nick:   sql.NullString{String: params.Nick, Valid: true},
		Kind:   params.Kind,
		Text:   sql.NullString{String: params.Text, Valid: true},
	})
}
