{
  description = "labcoat simple example NixOS system flake";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-24.05";
  };

  outputs = { nixpkgs, ... }:
    let
      inherit (nixpkgs) lib;
    in
    {
      nixosConfigurations =
        let
          # Configuration shared by all systems.
          commonModule = {
            services.openssh = {
              enable = true;

              # By default, labcoat uses the `root` user to SSH into and deploy
              # hosts.
              settings.PermitRootLogin = "yes";
            };

            system.stateVersion = "24.05";
          };
        in
        {
          # A NixOS system named `mysystem`.  This attribute name will be used
          # in labcoat's host list.
          mysystem = lib.nixosSystem {
            system = "x86_64-linux";

            modules = [
              {
                # By default, labcoat uses the `networking.fqdnOrHostName`
                # attribute value when connecting to a host.  NixOS constructs
                # this value by combining `hostName` with the optional `domain`
                # value below.
                networking = {
                  hostName = "mysystem";
                  # domain = "home.arpa";
                };
              }
              commonModule
              ./hardware-configuration.nix
            ];
          };
        };
    };
}
