package handlers

import (
	"goirc/internal/responder"
	"goirc/model"
)

func DeferredDelivery(params responder.Responder) error {
	if params.Target() == params.Nick() {
		params.Privmsgf(params.Target(), "not your personal secretary")
		return nil
	}

	prefix := params.Match(1)
	message := params.Match(2)

	// if the prefix matches a currently joined nick, we do nothing
	if model.PrefixMatchesJoinedNick(model.DB, params.Target(), prefix) {
		return nil
	}

	if prefix == "annie" {
		return nil
	}

	if model.PrefixMatchesKnownNick(model.DB, params.Target(), prefix) {
		_, err := model.DB.Exec(`insert into laters values(datetime('now'), ?, ?, ?, ?)`, params.Nick(), prefix, message, false)
		if err != nil {
			return err
		}

		params.Privmsgf(params.Target(), "%s: will send to %s* later", params.Nick(), prefix)
	}
	return nil
}
