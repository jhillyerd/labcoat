# Builds nixosConfigurations flake output.
{ nixpkgs, ... }@inputs: catalog:
let
  inherit (nixpkgs.lib) mapAttrs nixosSystem splitString;

  # Authorized SSH keys for all systems.
  authorizedKeys = splitString "\n" (builtins.readFile ../authorized_keys.txt);

  # Shared utility functions for use in host configs.
  util = import ./util.nix { lib = nixpkgs.lib; };

  # Creates a nixosSystem attribute set for the specified node, allowing the
  # node config to be overridden.  This can be used to generate alternate
  # versions of the host config & hardware, ie for VM or SD card images.
  mkSystem =
    { hostName
    , node
    , hardware ? node.hw
    , modules ? [ ]
    }:
    nixosSystem {
      system = node.system;

      # `specialArgs` allows access to catalog, environment, etc with
      # hosts and roles.  `self` lets a host reference aspects of
      # itself.
      specialArgs = inputs // {
        inherit authorizedKeys catalog hostName util;
        self = node;
      };

      modules = modules ++ [
        (nodeModule node)
        hardware
        node.config
      ];
    };

  # Common system config built from node entry.
  nodeModule = node: { hostName, ... }: {
    # By default, labcoat uses the `networking.fqdnOrHostName`
    # attribute value when connecting to a host.  NixOS constructs
    # this value by combining `hostName` with the optional `domain`
    # value below.
    networking = {
      inherit hostName;
      domain = "home.arpa";
      hostId = node.hostId or null;
    };
  };
in
# Builds nixosConfiguration by applying mkSystem function to each node in the catalog.
mapAttrs
  (hostName: node:
  mkSystem { inherit hostName node; })
  catalog.nodes
