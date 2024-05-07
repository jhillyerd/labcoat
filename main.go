package main

import (
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jhillyerd/labui/internal/ui"
)

func main() {
	// Init logging.
	lf, err := tea.LogToFile("debug.log", "")
	if err != nil {
		fmt.Println("fatal: ", err)
		os.Exit(1)
	}
	defer lf.Close()

	slog.Info("### STARTUP ###################################################################")

	p := tea.NewProgram(ui.New(flakeHosts()), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("oops: %v", err)
		os.Exit(1)
	}
}

func flakeHosts() []string {
	return []string{"fastd", "metrics", "longlonglonglonglonglonglonglonglonglong", "web"}
}
