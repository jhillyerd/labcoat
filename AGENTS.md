# AGENTS.md

Guidelines for AI coding agents working in the labcoat codebase.

## Project Overview

Labcoat is a TUI application for deploying flake-based NixOS systems, built with Go 1.25, Bubble Tea (charm.land/bubbletea/v2), Lipgloss, BoltDB, and TOML configuration.

## Build Commands

```bash
go build -o labcoat .           # Build binary
```

## Test Commands

```bash
go test ./...                   # Run all tests
go test -v ./...                # Verbose output
go test ./internal/runner       # Tests in specific package
go test -run TestFormatOutput ./internal/runner    # Single test by name
go test -v -run TestBufContent ./internal/runner   # Single test, verbose
go test -cover ./...            # With coverage
```

## Lint Commands

```bash
golint ./...                    # Run golint (in nix develop)
go vet ./...                    # Run go vet
go fmt ./...                    # Format code
```

## Code Style Guidelines

### Imports

Group imports with blank lines: standard library, third-party (Charm ecosystem), then local packages:

```go
import (
	"context"
	"fmt"
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/pelletier/go-toml/v2"

	"github.com/jhillyerd/labcoat/internal/config"
)
```

### Naming Conventions

- Packages: lowercase, single word (config, nix, runner, store, ui)
- Types/Functions: PascalCase exported, camelCase unexported
- Acronyms: keep consistent (SSHDestination, not SshDestination)
- Interface methods: descriptive verbs (Get, Load, Write, Run)

### Struct Definitions

Group related fields with blank lines; order by logical grouping:

```go
type Model struct {
	// Configuration and context
	ctx    context.Context
	config config.Config

	// UI state
	viewMode  int
	ready     bool

	// Data
	hosts        map[string]*hostModel
	selectedHost *hostModel
}
```

### Error Handling

Return errors rather than panicking. Wrap with fmt.Errorf and %w:

```go
if err != nil {
	return nil, fmt.Errorf("nix decode failed: %w\n\nJSON output:\n%s", err, string(output))
}
```

### Logging

Use log/slog with structured logging (Debug, Info, Warn, Error):

```go
slog.Info("Fetching target info", "host", host.name, "worker", worker)
```

### Testing

Use stretchr/testify - require for setup, assert for conditions:

```go
func TestMyFunction(t *testing.T) {
	result, err := DoSomething()
	require.NoError(t, err)
	assert.Equal(t, expected, result.Value)
}
```

Prefer table-driven tests:

```go
tcs := map[string]struct{ in, want string }{
	"empty": {in: "", want: ""},
	"plain": {in: "hello", want: "hello"},
}
for name, tc := range tcs {
	tc := tc
	t.Run(name, func(t *testing.T) {
		assert.Equal(t, tc.want, MyFunc(tc.in))
	})
}
```

### Comments

Doc comments for exported items, starting with the name:

```go
// Load parses the config file at path, overlaying values onto Default.
func Load(path string, mustExist bool) (*Config, error) {
```

### Bubble Tea Patterns

- Use tea.Cmd for async operations, tea.Batch for multiple commands
- Define message types as unexported structs
- Use type switch in Update functions

```go
type hostChangedMsg struct{ hostName string }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case hostChangedMsg:
		return m, m.handleHostChangedMsg(msg)
	}
	return m, nil
}
```

## Project Structure

```
labcoat/
├── main.go              # Entry point, CLI parsing
├── internal/
│   ├── config/          # Configuration loading and types
│   ├── nix/             # Nix command execution and parsing
│   ├── npool/           # Worker pool for concurrent nix operations
│   ├── runner/          # Command runner with output buffering
│   ├── store/           # BoltDB persistence layer
│   └── ui/              # Bubble Tea TUI components
├── examples/            # Example NixOS flake configurations
└── flake.nix            # Nix build configuration
```
