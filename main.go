package main

import (
	"context"
	"fmt"
	"goirc/bot"
	"goirc/events"
	db "goirc/model"
	"goirc/util"
	"goirc/web"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rcy/evoke"
)

//go:generate go tool github.com/sqlc-dev/sqlc/cmd/sqlc generate --file db/sqlc.yaml

func main() {
	evokeFile, ok := os.LookupEnv("EVOKE_DB")
	if !ok {
		log.Fatal("EVOKE_DB not defined")
	}
	es, err := evoke.NewStore(evoke.Config{DBFile: evokeFile})
	if err != nil {
		log.Fatal(err)
	}
	defer es.Close()

	b, err := bot.Connect(
		es,
		util.Getenv("IRC_NICK"),
		util.Getenv("IRC_CHANNEL"),
		util.Getenv("IRC_SERVER"),
		util.Getenv("SASL_LOGIN"),
		util.Getenv("SASL_PASSWORD"))
	if err != nil {
		log.Fatal(err)
	}

	addHandlers(b)

	go b.Loop()
	go web.Serve(db.DB, b, es)

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	<-ctx.Done()
	es.MustInsert(b.Channel, events.BotQuit{Nick: b.Conn.GetNick()})

	fmt.Println("Clean shutdown.")
}
