package piss

import (
	"fmt"

	"goirc/internal/responder"

	"github.com/rcy/gopiss"
)

func Handle(params responder.Responder) error {
	level, err := gopiss.GetISSUrineTankLevel()
	if err != nil {
		return fmt.Errorf("couldn't get piss tank level: %w", err)
	}

	params.Privmsgf(params.Target(), "ISS urine tank level: %.1f%%", level)
	return nil
}
