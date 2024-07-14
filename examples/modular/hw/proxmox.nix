# Example Proxmox VE guest hardware config.
{ lib, modulesPath, ... }: {
  boot.loader.grub.enable = true;
  boot.loader.grub.device = "/dev/sda";

  imports = [ (modulesPath + "/profiles/qemu-guest.nix") ];

  boot = {
    initrd.availableKernelModules = [ "uas" ];
    initrd.kernelModules = [ ];

    kernelModules = [ ];
    extraModulePackages = [ ];

    kernelParams = [ "console=ttyS0" ];
  };

  fileSystems."/" = {
    device = "/dev/disk/by-label/nixos";
    fsType = "ext4";
  };

  swapDevices = [ ];

  networking.useDHCP = false;

  systemd.network = {
    enable = true;

    networks."10-cluster" = {
      matchConfig.Name = "enp0s18";
      networkConfig = {
        DHCP = lib.mkDefault "ipv4";
        IPv6AcceptRA = lib.mkDefault "no";
        LinkLocalAddressing = lib.mkDefault "no";
      };
    };
  };

  nix.settings.max-jobs = lib.mkDefault 2;

  services.qemuGuest.enable = true;
}
