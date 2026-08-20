package main

import (
	"context"
	"fmt"
	"goirc/bot"
	"goirc/config"
	"goirc/events"
	"goirc/handlers/lua"
	"goirc/handlers/mcp"
	"goirc/internal/ai"
	internalmcp "goirc/internal/mcp"
	db "goirc/model"
	"goirc/web"
	"log"
	"os/signal"
	"syscall"

	"github.com/rcy/evoke"
)

//go:generate go tool github.com/sqlc-dev/sqlc/cmd/sqlc generate --file db/sqlc.yaml

func main() {
	cfg := config.Load()

	es, err := evoke.NewStore(evoke.Config{DBFile: cfg.EvokeDB})
	if err != nil {
		log.Fatal(err)
	}
	defer es.Close()

	err = lua.CloneOrPull()
	if err != nil {
		log.Fatal(err)
	}

	b, err := bot.Connect(
		es,
		cfg.IRCNick,
		cfg.IRCChannel,
		cfg.IRCServer,
		cfg.SASLLogin,
		cfg.SASLPassword)
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
