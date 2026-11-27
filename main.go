package main

import (
	"context"
	"fmt"
	"goirc/bot"
	"goirc/events"
	"goirc/handlers/lua"
	"goirc/handlers/mcp"
	"goirc/internal/ai"
	internalmcp "goirc/internal/mcp"
	db "goirc/model"
	appriver "goirc/river"
	"goirc/util"
	"goirc/web"
	"goirc/workers/noop"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/rcy/evoke"
	"github.com/riverqueue/river"
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
	defer func() { _ = es.Close() }()

	err = lua.CloneOrPull()
	if err != nil {
		log.Fatal(err)
	}

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

	// Initialize MCP client
	mcpClient, err := internalmcp.FromEnv(context.Background())
	if err != nil {
		log.Printf("MCP: %v", err)
	}
	if mcpClient != nil {
		mcp.SetClient(mcpClient)
		ai.SetMCPClient(mcpClient)
		log.Printf("MCP: connected to %d tool(s)", len(mcpClient.ListTools()))
	}

	addHandlers(b)

	// River job queue
	dataDir := filepath.Dir(evokeFile)
	workers := river.NewWorkers()
	river.AddWorker(workers, &noop.JobWorker{})

	riverClient, err := appriver.New(dataDir, workers)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	if err := riverClient.Start(ctx); err != nil {
		log.Fatal(err)
	}

	go b.Loop()
	go web.Serve(db.DB, b, es, riverClient)

	<-ctx.Done()
	es.MustInsert(b.Channel, events.BotQuit{Nick: b.Conn.GetNick()})
	_ = riverClient.Stop(context.Background())

	fmt.Println("Clean shutdown.")
}
