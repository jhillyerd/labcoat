# Catalog defines the systems & services on my network.
{ system }: rec {
  nodes = import ./nodes.nix { inherit system; };
}
