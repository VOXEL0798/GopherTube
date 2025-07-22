package app

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/urfave/cli/v3"
)

var version = "dev"

//go:embed description.txt
var Desc string

func New() cli.Command {
	return cli.Command{
		Name: "GopherTube",
		Authors: []any{
			"KrishnaSSH <krishna.pytech@gmail.com>",
		},
		Usage:       "Terminal YouTube Search & Play",
		Description: Desc,
		Flags:       Flags(),
		Version:     version,
		Action:      Action,
	}
}

// Action is the equivalent of the main except that all flags/configs
// have already been parsed and sanitized.
func Action(ctx context.Context, cmd *cli.Command) error {
	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\033[1;33mExiting...\033[0m")
		os.Exit(0)
	}()

	for {
		mainMenu := []string{"Search YouTube", "Search Downloads"}
		action := exec.Command("fzf", "--prompt=Select mode: ")
		action.Stdin = strings.NewReader(strings.Join(mainMenu, "\n"))
		out, _ := action.Output()
		choice := strings.TrimSpace(string(out))
		switch choice {
		case "Search YouTube":
			gophertubeYouTubeMode(cmd)
		case "Search Downloads":
			gophertubeDownloadsMode(cmd)
		default:
			return nil
		}
	}
}
