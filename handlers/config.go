package handlers

import (
	"context"
	"goirc/db/model"
	"goirc/internal/responder"
	db "goirc/model"
	"strings"
)

func GetConfig(params responder.Responder) error {
	key := params.Match(1)

	q := model.New(db.DB)
	cfg, err := q.GetConfig(context.TODO(), key)
	if err != nil {
		params.Privmsgf(params.Target(), "%s: %s not found", params.Nick(), key)
		return nil
	}

	params.Privmsgf(params.Target(), "%s: %s = %s", params.Nick(), cfg.Key, cfg.Value)
	return nil
}

func SetConfig(params responder.Responder) error {
	key := params.Match(1)
	value := params.Match(2)

	q := model.New(db.DB)
	ctx := context.TODO()

	if strings.HasPrefix(value, "$") {
		refKey := value[1:]
		ref, err := q.GetConfig(ctx, refKey)
		if err != nil {
			params.Privmsgf(params.Target(), "%s: %s is not set", params.Nick(), refKey)
			return nil
		}
		value = ref.Value
	}

	prev, err := q.GetConfig(ctx, key)

	err = q.SetConfig(ctx, model.SetConfigParams{
		Key:   key,
		Value: value,
		Nick:  params.Nick(),
	})
	if err != nil {
		return err
	}

	if prev.Value != "" {
		params.Privmsgf(params.Target(), "%s: %s = %s (was %s)", params.Nick(), key, value, prev.Value)
	} else {
		params.Privmsgf(params.Target(), "%s: %s = %s", params.Nick(), key, value)
	}
	return nil
}
