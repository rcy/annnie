package annie

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"goirc/db/model"
	"goirc/internal/ai"
	"goirc/internal/responder"
	db "goirc/model"
	"strings"
)

func Handle(params responder.Responder) error {
	ctx := params.Context()

	if len(params.Matches()) < 2 {
		return nil
	}
	msg := strings.TrimSpace(params.Matches()[1])

	q := model.New(db.DB.DB)

	override, err := GetSystemOverride(ctx)
	if err != nil {
		return fmt.Errorf("getSystemOverride: %w", err)
	}

	kind, err := ai.Complete(ctx, RoutingPrompt, msg)
	if err != nil {
		return err
	}

	systemPrompt, err := BuildSystemPrompt(ctx, q, kind, override)
	if err != nil {
		return fmt.Errorf("buildSystemPrompt: %w", err)
	}

	if systemPrompt == "" {
		params.Privmsgf(params.Target(), "%s: [interpreted '%s' as an unknown type: %s]", params.Nick(), msg, kind)
		return nil
	}

	if kind == "statement" {
		_, err = q.InsertNote(ctx, model.InsertNoteParams{
			Target: params.Target(),
			Nick:   sql.NullString{String: params.Nick(), Valid: true},
			Kind:   "note",
			Text:   sql.NullString{String: msg, Valid: true},
		})
		if err != nil {
			return err
		}
	}

	response, err := ai.Complete(ctx, systemPrompt, msg)
	if err != nil {
		return err
	}
	params.Privmsgf(params.Target(), "%s: %s", params.Nick(), response)

	return nil
}

func BuildSystemPrompt(ctx context.Context, q *model.Queries, kind, override string) (string, error) {
	switch kind {
	case "statement":
		return fmt.Sprintf(`you are annnie, a friend hanging out in an irc channel. given the following statement, reflect on its meaning, and come up with a terse response, no more than a short sentence, in lower case, with minimal punctuation (commas are ok)

%s`, override), nil

	case "question":
		notes, err := q.NonAnonNotes(ctx)
		if err != nil {
			return "", err
		}
		lines := make([]string, len(notes))
		for i, n := range notes {
			lines[i] = fmt.Sprintf("%s <%s> %s", n.CreatedAt, n.Nick.String, n.Text.String)
		}
		return fmt.Sprintf(`You are annnie, a friend hanging out in an irc channel.
You have been asked a question. Read the question, and think about it in the context of all you have read in this channel.
Respond with single sentences, in lower case, with minimal punctuation (commas are ok).
Do not refer to yourself in the third person.

%s
%s`, override, strings.Join(lines, "\n")), nil

	case "pleasantry":
		return fmt.Sprintf(`You are annnie, a friend hanging out in an irc channel.
Someone has posted some pleasantry or small talk.
Respond in kind, but in a very uninterested dismissive way.
Respond in lower case, with minimal punctuation (commas are ok).

%s`, override), nil
	}

	return "", nil
}

func GetSystemOverride(ctx context.Context) (string, error) {
	cfg, err := model.New(db.DB.DB).GetConfig(ctx, "system")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("GetConfig: %w", err)
	}
	return cfg.Value, nil
}

const RoutingPrompt = "categorize input into statements, questions, or pleasantries. Questions include direct questions and requests for information or action. If it is a statement, reply with the one word 'statement'. If it is a question or request for information or action, reply with 'question'. If it is a pleasantry, reply with 'pleasantry'.  Do not make any tool calls to help make this determination."
