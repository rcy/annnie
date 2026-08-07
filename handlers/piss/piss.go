package piss

import (
	"context"
	"fmt"
	"log"
	"math"

	"goirc/internal/responder"
	"goirc/pubsub"

	"github.com/rcy/gopiss"
)

func Handle(params responder.Responder) error {
	level, err := gopiss.GetISSUrineTankLevel()
	if err != nil {
		return fmt.Errorf("couldn't get piss tank level: %w", err)
	}

	params.Privmsgf(params.Target(), "the iss urine tank level is at %.0f%%", level)
	return nil
}

// StartWatcher connects to the ISS urine tank telemetry and publishes
// the integer tank level via pubsub on each change.
func StartWatcher(ctx context.Context) {
	ch, err := gopiss.WatchISSUrineTankLevel(ctx)
	if err != nil {
		log.Printf("piss watcher: %v", err)
		return
	}

	go func() {
		var lastIntLevel int

		for level := range ch {
			currentInt := int(math.Floor(level))
			if currentInt != lastIntLevel {
				pubsub.Publish("piss", currentInt)
				lastIntLevel = currentInt
			}
		}
	}()
}
