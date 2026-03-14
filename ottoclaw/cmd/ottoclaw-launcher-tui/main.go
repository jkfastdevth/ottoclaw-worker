package main

import (
	"fmt"
	"os"

	"github.com/sipeed/ottoclaw/cmd/ottoclaw-launcher-tui/internal/ui"
)

func main() {
	if err := ui.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
