package handlers

import (
	"goirc/configs"
	"goirc/internal/responder"
	"strings"
)

func GetConfig(params responder.Responder) error {
	key := params.Match(1)

	value, err := configs.GetLiteral(key)
	if err != nil {
		params.Privmsgf(params.Target(), "%s: %s not found", params.Nick(), key)
		return nil
	}

	params.Privmsgf(params.Target(), "%s: %s = %s", params.Nick(), key, value)
	return nil
}

func SetConfig(params responder.Responder) error {
	key := params.Match(1)
	value := params.Match(2)

	if strings.HasPrefix(value, "$") {
		ref, err := configs.GetLiteral(value[1:])
		if err != nil {
			params.Privmsgf(params.Target(), "%s: %s is not set", params.Nick(), value[1:])
			return nil
		}
		value = ref
	}

	prev, err := configs.Set(key, value, params.Nick())
	if err != nil {
		return err
	}

	if prev != "" {
		params.Privmsgf(params.Target(), "%s: %s = %s (was %s)", params.Nick(), key, value, prev)
	} else {
		params.Privmsgf(params.Target(), "%s: %s = %s", params.Nick(), key, value)
	}
	return nil
}
