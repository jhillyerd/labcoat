package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jhillyerd/labui/internal/config"
	"github.com/jhillyerd/labui/internal/nix"
	"github.com/jhillyerd/labui/internal/ui"
)

func main() {
	var (
		help     = flag.Bool("help", false, "Print argument help message")
		defaults = flag.Bool("defaults", false, "Prints default configuration to stdout and exits")
		logPath  = flag.String("log", "", "File to write debug logs to")
	)
	flag.Parse()

	if *help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if *defaults {
		config.PrintDefaults()
		os.Exit(0)
	}

	// Load config file if present.
	configRoot := os.Getenv("XDG_CONFIG_HOME")
	if configRoot == "" {
		home := os.Getenv("HOME")
		if home == "" {
			fmt.Fprintln(os.Stderr, "Neither $XDG_CONFIG_HOME or $HOME available")
			os.Exit(1)
		}
		configRoot = filepath.Join(home, ".config")
	}

	configPath := filepath.Join(configRoot, "labui", "config.toml")
	conf, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config %q: %v\n", configPath, err)
		os.Exit(1)
	}

	// Init logging.
	if *logPath != "" {
		lf, err := tea.LogToFile(*logPath, "")
		if err != nil {
			fmt.Fprintln(os.Stderr, "logging error: ", err)
			os.Exit(1)
		}
		defer lf.Close()

		slog.SetLogLoggerLevel(slog.LevelDebug)
		slog.Info("### STARTUP ###################################################################")
	} else {
		// Prevent log output corrupting UI.
		log.SetOutput(io.Discard)
	}

	// Load host list from flake.
	flakePath := flag.Arg(0)
	if flakePath == "" {
		flakePath, err = os.Getwd()
	} else {
		flakePath, err = filepath.Abs(flakePath)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	hosts, nerr := nix.GetNames(nix.NamesRequest{FlakePath: flakePath})
	if nerr != nil {
		fmt.Fprintln(os.Stderr, nerr.Error())
		os.Exit(1)
	}

	// Launch UI.
	p := tea.NewProgram(ui.New(*conf, config.DefaultKeyMap, flakePath, hosts), tea.WithAltScreen())
	go p.Send(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
