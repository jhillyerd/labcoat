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
	"github.com/jhillyerd/labcoat/internal/config"
	"github.com/jhillyerd/labcoat/internal/nix"
	"github.com/jhillyerd/labcoat/internal/ui"
)

func main() {
	var (
		help          = flag.Bool("help", false, "Print argument help message")
		configPathOpt = flag.String("config", "", "Path to TOML config file")
		defaults      = flag.Bool("defaults", false, "Prints default configuration to stdout and exits")
		logPath       = flag.String("log", "", "File to write debug logs to")
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

	// Load config file if present.
	var configPath string
	var configMustExist bool

	if *configPathOpt == "" {
		configPath = defaultConfigPath()
	} else {
		configPath = *configPathOpt
		configMustExist = true
	}

	conf, err := config.Load(configPath, configMustExist)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config: %v\n", err)
		os.Exit(1)
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
	if len(hosts) == 0 {
		fmt.Fprintln(os.Stderr, "No hosts (nixosConfigurations) found in flake")
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

func defaultConfigPath() string {
	configRoot := os.Getenv("XDG_CONFIG_HOME")
	if configRoot == "" {
		home := os.Getenv("HOME")
		if home == "" {
			fmt.Fprintln(os.Stderr, "Neither $XDG_CONFIG_HOME or $HOME available")
			os.Exit(1)
		}
		configRoot = filepath.Join(home, ".config")
	}

	return filepath.Join(configRoot, "labcoat", "config.toml")
}
