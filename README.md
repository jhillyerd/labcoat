# labui

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


## Contributing

Contributions are welcome, with the following provisions:

1. If you are not already familiar with *The Elm Architecture*, please read
   through the [Bubble Tea Tutorial] to learn basics.
2. Please create a new Issue before starting work on large changes, to
   make sure they fit my vision for the project.
3. I am not interested in supporting non-Nix tools, such as Ansible or Puppet.
   Please feel free to fork if that is your goal!


[Bubble Tea Tutorial]: https://github.com/charmbracelet/bubbletea/tree/master/tutorials/basics
