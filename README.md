# labcoat

labcoat is a TUI for deploying flake based [NixOS] systems.  It lets you select
from a list of `nixosConfigurations` available in your flake, giving you one
touch access to deploy and inspect those systems.

labcoat is ideal for managing NixOS lab environments up to a couple dozen
systems; particularly during the development phase where you do not yet
know if your configuration works correctly.

For production NixOS environments with many instances of the same configuration,
you will be better served by a parallel deployment tool such as [Colmena].


## Features

- [x] Automatically fetch node list from nix flake
- [x] Fetch individual node deploy configs (ie FQDN) from flake
- [x] Fetch target host status on hover
- [x] Build & deploy nix configuration to target host
- [x] Launch interactive SSH into target host
- [x] Reboot target host with confirmation
  - [ ] Use ping to track host status during reboot
- [x] Run specified command on target host
- [ ] Run configurable commands on target host, w/ optional confirmation
- [ ] Record/display per-node command and deployment history
- [ ] Gather target host deployment/generation state
  - [ ] Flag out-of-date hosts in list UI
- [x] External pager support


## Status

labcoat is currently incomplete, and alpha quality.  However, it's good enough
to manage [my homelab], and maybe yours too.


## Usage

From the directory containing your systems flake, execute:

```sh
nix run github:jhillyerd/labcoat
```

Alternately you may pass the path to a directory containing flake.nix as
the last argument

```sh
nix run github:jhillyerd/labcoat -- ~/myflake/
```

## Configuration

labcoat contains a default configuration, and does not require a configuration
file.  The `-defaults` argument will print the default configuration to
`stdout`.

If you want to make changes to the default configuration, it should be stored
in `$XDG_HOME/labcoat/config.toml`.  labcoat always layers your configuration
on top of it's defaults, so you may remove or comment out anything you don't
wish to change.

Create the TOML config file:

```sh
mkdir -p ~/.config/labcoat
nix run github:jhillyerd/labcoat -- -defaults > ~/.config/labcoat/config.toml
```


## Contributing

Contributions are welcome, with the following provisions:

1. If you are not already familiar with *The Elm Architecture*, please read
   through the [Bubble Tea Tutorial] to learn basics.
2. Please create a new Issue before starting work on large changes, to
   make sure they fit my vision for the project.
3. I am not interested in supporting non-Nix tools, such as Ansible or Puppet.
   Please feel free to fork if that is your goal!


[Bubble Tea Tutorial]: https://github.com/charmbracelet/bubbletea/tree/master/tutorials/basics
[Colmena]:             https://github.com/zhaofengli/colmena
[my homelab]:          https://github.com/jhillyerd/homelab
[NixOS]:               https://nixos.org/
