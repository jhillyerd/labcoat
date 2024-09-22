{
  description = "labcoat vagrant-demo system flake";

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
            boot.loader.grub.enable = true;
            boot.loader.grub.device = "/dev/vda";

            boot.initrd.checkJournalingFS = false;

            services.openssh = {
              enable = true;
              settings.PermitRootLogin = "yes";
            };

            system.stateVersion = "24.05";
          };

          # Creates a network module based on provided config.
          networkModule = { hostName, ipAddr }: {
            _module.args.deployAddr = ipAddr; # See labcoat deploy-host-attr config.
            networking = {
              inherit hostName;
              interfaces = {
                ens6.ipv4.addresses = [{
                  address = ipAddr;
                  prefixLength = 24;
                }];
              };
            };
          };
        in
        {
          db-lab = lib.nixosSystem {
            system = "x86_64-linux";
            modules = [
              commonModule
              ./hardware-configuration.nix
              (networkModule { hostName = "db-lab"; ipAddr = "192.168.33.10"; })
            ];
          };

          web-lab = lib.nixosSystem {
            system = "x86_64-linux";
            modules = [
              commonModule
              ./hardware-configuration.nix
              (networkModule { hostName = "web-lab"; ipAddr = "192.168.33.11"; })
            ];
          };

          # Fake systems to pad the labcoat host list.
          fake-host-1 = lib.nixosSystem {
            system = "x86_64-linux";
            modules = [
              commonModule
              ./hardware-configuration.nix
              (networkModule { hostName = "fake-host-1"; ipAddr = "192.168.33.11"; })
            ];
          };

          fake-host-2 = lib.nixosSystem {
            system = "x86_64-linux";
            modules = [
              commonModule
              ./hardware-configuration.nix
              (networkModule { hostName = "fake-host-2"; ipAddr = "192.168.33.10"; })
            ];
          };
        };
    };
}
