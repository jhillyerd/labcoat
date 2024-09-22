# vagrant-demo

Used to generated screen recordings of labcoat.

## Usage

Warning: The `Makefile` will make changes to your SSH `known_hosts` file.

```sh
# Setup VMs
make prepare

# Generate recordings
make

# Teardown VMs
make destroy
```
