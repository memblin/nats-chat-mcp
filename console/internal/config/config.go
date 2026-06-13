// Package config resolves the console's runtime configuration from CLI flags,
// environment variables, and built-in defaults — in that priority order.
package config

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// Defaults applied when neither a flag nor an env var supplies a value.
const (
	DefaultNatsURL  = "nats://localhost:4222"
	DefaultIdentity = "operator"
)

// Config is the fully-resolved console configuration.
type Config struct {
	NatsURL     string // NATS server URL
	Identity    string // display name in rooms
	DefaultRoom string // room to join on startup ("" = none)
}

// Resolve parses args (typically os.Args[1:]) and overlays them on environment
// variables and defaults. Priority: flag > env var > default.
func Resolve(args []string) (Config, error) {
	fs := flag.NewFlagSet("nats-chat-console", flag.ContinueOnError)
	natsURL := fs.String("nats-url", "", "NATS server URL (env NATS_URL)")
	identity := fs.String("identity", "", "your display name in rooms (env NATS_IDENTITY)")
	room := fs.String("room", "", "room to join on startup (env NATS_DEFAULT_ROOM)")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	return Config{
		NatsURL:     firstNonEmpty(*natsURL, os.Getenv("NATS_URL"), DefaultNatsURL),
		Identity:    firstNonEmpty(*identity, os.Getenv("NATS_IDENTITY"), DefaultIdentity),
		DefaultRoom: firstNonEmpty(*room, os.Getenv("NATS_DEFAULT_ROOM"), ""),
	}, nil
}

// firstNonEmpty returns the first argument that is not the empty string.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// String renders the resolved config for the startup banner.
func (c Config) String() string {
	room := c.DefaultRoom
	if room == "" {
		room = "(none)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  nats-url: %s\n", c.NatsURL)
	fmt.Fprintf(&b, "  identity: %s\n", c.Identity)
	fmt.Fprintf(&b, "  room:     %s", room)
	return b.String()
}
