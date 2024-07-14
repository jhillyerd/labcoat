# Modular Example

This is a NixOS system flake example for a network of machines, with support
for differing hardware profiles.

This example was extracted from [my homelab configuration](
https://github.com/jhillyerd/homelab/tree/main/nixos), it demonstrates a
potential way to define a set of machines for labcoat to deploy, but following
this pattern is not mandatory.

To add a new host to this configuration, you would:

1. Add it's hardware-configuration.nix to the `hw` directory (if unique)
2. Add it's configuration.nix to the `hosts` directory
3. Add a new entry referencing the two configs above to [`catalog/nodes.nix`](
   https://github.com/jhillyerd/labcoat/blob/main/examples/modular/catalog/nodes.nix)
