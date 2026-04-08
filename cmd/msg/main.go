// Package main provides the msg CLI entry point for agent messaging.
package main

import (
	"fmt"
	"os"

	"jack.dev/jack/msg"
)

func main() {
	msg.Homeserver = os.Getenv("JACK_HOMESERVER")
	if msg.Homeserver == "" {
		fmt.Fprintln(os.Stderr, "JACK_HOMESERVER environment variable is required")
		os.Exit(1)
	}
	msg.DataDir = os.Getenv("JACK_DATA_DIR")

	if err := msg.Cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
