// Command nats-console is a standalone terminal chat client for the nats-chat
// system. It connects directly to the same NATS JetStream server the MCP server
// uses and participates as a first-class human identity: joining rooms, reading
// and sending messages, and appearing in list_agents via the presence registry.
package main

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/memblin/nats-chat-mcp/console/internal/config"
	natsclient "github.com/memblin/nats-chat-mcp/console/internal/nats"
	"github.com/memblin/nats-chat-mcp/console/internal/ui"
)

func main() {
	cfg, err := config.Resolve(os.Args[1:])
	if err != nil {
		// flag.ContinueOnError has already written the usage message.
		os.Exit(2)
	}

	fmt.Println("nats-console — resolved configuration:")
	fmt.Println(cfg.String())

	identity := natsclient.NewIdentity(cfg.Identity)

	// The event sink forwards async NATS events into the Bubbletea program. The
	// program is created after the client, so guard it behind an atomic pointer
	// that NATS callback goroutines read safely.
	var prog atomic.Pointer[tea.Program]
	sink := func(ev any) {
		if p := prog.Load(); p != nil {
			p.Send(ev)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Printf("connecting to %s as %q (id %s)…\n", cfg.NatsURL, identity.Name, identity.ID)
	client, err := natsclient.Connect(ctx, cfg.NatsURL, identity, sink)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("connected — starting console…")

	model := ui.New(cfg, client)
	p := tea.NewProgram(model, tea.WithAltScreen())
	prog.Store(p)

	go client.StartHeartbeat(ctx)
	go client.StartPresencePoll(ctx)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}

	cancel()
	client.Close(context.Background())
}
