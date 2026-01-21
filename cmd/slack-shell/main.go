package main

import (
	"fmt"
	"os"

	"github.com/polidog/slack-shell/internal/app"
	"github.com/polidog/slack-shell/internal/config"
	"github.com/polidog/slack-shell/internal/version"
)

func main() {
	// Check for version command
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println(version.String())
		return
	}

	// Check for logout command
	if len(os.Args) > 1 && os.Args[1] == "logout" {
		if err := app.Logout(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Check for config command
	if len(os.Args) > 1 && os.Args[1] == "config" {
		if len(os.Args) > 2 && os.Args[2] == "init" {
			// Parse arguments: config init [path] [--force|-f]
			var path string
			var force bool
			for _, arg := range os.Args[3:] {
				if arg == "--force" || arg == "-f" {
					force = true
				} else if path == "" {
					path = arg
				}
			}

			configPath, err := config.InitConfig(path, force)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Config file created at %s\n", configPath)
			return
		}
		// Show config subcommand help
		fmt.Println("Usage: slack-shell config <subcommand>")
		fmt.Println("")
		fmt.Println("Subcommands:")
		fmt.Println("  init [path] [--force]  Create a sample config file")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  slack-shell config init                    # Create at ~/.slack-shell/config.yaml")
		fmt.Println("  slack-shell config init ~/work.yaml        # Create at specified path")
		fmt.Println("  slack-shell config init ~/work.yaml -f     # Overwrite if exists")
		return
	}

	// Check for -c option (execute command and exit)
	if len(os.Args) > 2 && os.Args[1] == "-c" {
		command := os.Args[2]
		application, err := app.New(app.WithNonInteractive())
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
