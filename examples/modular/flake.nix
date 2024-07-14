{
  description = "labcoat example of a modular NixOS systems flake";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-24.05";
    nixpkgs-unstable.url = "github:nixos/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    { flake-utils, ... }@inputs:
    let
      # catalog.nodes defines the systems available in this flake.
      catalog = import ./catalog { inherit (flake-utils.lib) system; };
    in
    {
      # Convert catalog.nodes into a set of NixOS configs.
      nixosConfigurations = import ./nix/nixos-configurations.nix inputs catalog;
    };
}
