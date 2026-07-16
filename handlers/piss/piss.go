package piss

import (
	"context"
	"fmt"
	"log"

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

// StartWatcher connects to the ISS urine tank telemetry and announces
// when the level is around 69%.
func StartWatcher(ctx context.Context, target string, privmsgf func(string, string, ...any)) {
	ch, err := gopiss.WatchISSUrineTankLevel(ctx)
	if err != nil {
		log.Printf("piss watcher: %v", err)
		return
	}

	go func() {
		for level := range ch {
			if level >= 68.5 && level <= 69.5 {
				privmsgf(target, "the iss urine tank is at %.1f%%", level)
			}
		}
	}()
}
