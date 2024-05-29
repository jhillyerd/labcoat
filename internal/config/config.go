package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Commands Commands `toml:"commands"`
	Hosts    Hosts    `toml:"hosts" comment:"Host deployment configuration. Nix attrs typically start with 'flake' or 'target'."`
}

type Commands struct {
	StatusCmds []string `toml:"status-cmds" comment:"List of commands to run to check host status"`
}

type Hosts struct {
	DefaultSSHDomain string `toml:"default-ssh-domain" comment:"Appended after '.' to bare hostnames"`
	DefaultSSHUser   string `toml:"default-ssh-user"`
	DeployHostAttr   string `toml:"deploy-host-attr" comment:"Nix attr path for SSH deploy target hostname"`
	DeployUserAttr   string `toml:"deploy-user-attr"`
}

// Default returns the default Config.
func Default() Config {
	return Config{
		Commands: Commands{
			StatusCmds: []string{
				"systemctl --failed",
				"uname -a",
				"uptime",
				"df -h -x tmpfs -x overlay",
			},
		},
		Hosts: Hosts{
			DefaultSSHUser: "root",
			DeployHostAttr: "target.config.networking.fqdnOrHostName",
		},
	}
}

// PrintDefaults renders default config as TOML to stdout.
func PrintDefaults() error {
	b, err := toml.Marshal(Default())
	if err != nil {
		return err
	}

	fmt.Println("# labui default configuration, only needed if you wish to make changes.")
	fmt.Println()
	_, err = os.Stdout.Write(b)
	return err
}

// Load and parse config, overlaying `Default` values.
func Load(path string) (*Config, error) {
	conf := Default()

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Use defaults when no config file present.
			return &conf, nil
		}
		return nil, err
	}

	if err = toml.Unmarshal(b, &conf); err != nil {
		return nil, err
	}

	return &conf, nil
}
