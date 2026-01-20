package main

import (
	"fmt"
	"os"

	"github.com/polidog/slack-shell/internal/app"
)

func main() {
	// Check for logout command
	if len(os.Args) > 1 && os.Args[1] == "logout" {
		if err := app.Logout(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Check for -c option (execute command and exit)
	if len(os.Args) > 2 && os.Args[1] == "-c" {
		command := os.Args[2]
		application, err := app.New()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer application.Stop()

		if err := application.RunCommand(command); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	application, err := app.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer application.Stop()

	if err := application.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
