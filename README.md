# labui

## Features

- [x] Automatically fetch node list from nix flake
- [x] Fetch individual node deploy configs (ie FQDN) from flake
- [x] Fetch target host status on hover
- [x] Build & deploy nix configuration to target host
- [x] Launch interactive SSH into target host
- [x] Reboot target host with confirmation
  - [ ] Use ping to track host status during reboot
- [ ] Run specified command on target host
- [ ] Run configurable commands on target host, w/ optional confirmation
- [ ] Record/display per-node command and deployment history
- [ ] Gather target host deployment/generation state
  - [ ] Flag out-of-date hosts in list UI
- [ ] External pager support
