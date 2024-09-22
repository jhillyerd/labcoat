package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	General  General  `toml:"general"`
	Commands Commands `toml:"commands"`
	Hosts    Hosts    `toml:"hosts" comment:"Host deployment configuration. Nix attrs typically start with 'flake' or 'target'."`
	Nix      Nix      `toml:"nix"`
}

type General struct {
	Pager string `toml:"pager"`
}

type Commands struct {
	StatusCmds []string `toml:"status-cmds" comment:"List of commands to run to display host status"`
}

type Hosts struct {
	DefaultSSHDomain string `toml:"default-ssh-domain" comment:"Appended after '.' to bare hostnames"`
	DefaultSSHUser   string `toml:"default-ssh-user"`
	DeployHostAttr   string `toml:"deploy-host-attr" comment:"Nix attr path for SSH deploy target hostname"`
	DeployUserAttr   string `toml:"deploy-user-attr"`
}

type Nix struct {
	DefaultBuildHost string `toml:"default-build-host" comment:"Default [user@]host to run Nix builds on"`
}

// Default returns the default Config.
func Default() Config {
	return Config{
		General: General{
			Pager: "less",
		},
		Commands: Commands{
			StatusCmds: []string{
				"date",
				"systemctl --failed",
				"nixos-rebuild --no-build-nix list-generations",
				"uname -a",
				"uptime",
				"df -h -x tmpfs -x overlay",
			},
		},
		Hosts: Hosts{
			DefaultSSHUser: "root",
			DeployHostAttr: "target.config.networking.fqdnOrHostName",
		},
		Nix: Nix{
			DefaultBuildHost: "localhost",
		},
	}
}

// PrintDefaults renders default config as TOML to stdout.
func PrintDefaults() error {
	b, err := toml.Marshal(Default())
	if err != nil {
		return err
	}

	fmt.Println("# labcoat default configuration, only needed if you wish to make changes.")
	fmt.Println()
	_, err = os.Stdout.Write(b)
	return err
}

// Load and parse config, overlaying `Default` values.
func Load(path string, mustExist bool) (*Config, error) {
	conf := Default()

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if mustExist {
				return nil, err
			}

			slog.Warn("Config file not found, using defaults", "path", path)
			return &conf, nil
		}
		return nil, err
	}

	if err = toml.Unmarshal(b, &conf); err != nil {
		return nil, err
	}

	slog.Debug("Loaded config", "path", path)
	return &conf, nil
}
