package annietts

import (
	"strings"

	"goirc/handlers/annie"
	"goirc/handlers/tts"
	"goirc/internal/responder"
)

func Handle(params responder.Responder) error {
	if len(params.Matches()) < 2 {
		return nil
	}
	msg := strings.TrimSpace(params.Matches()[0])

	response, err := annie.Complete(params.Context(), params, msg)
	if err != nil {
		return err
	}

	return tts.Speak(response, params)
}
