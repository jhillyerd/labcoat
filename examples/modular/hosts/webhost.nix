# A host configuration that can be shared by multiple nodes.
{ self, util, ... }: {
  imports = [ ../common/all.nix ];

  # Example role usage, you may also use regular NixOS services here.
  roles.webserver = {
    enable = true;
    greeting = "Hello from labcoat";
  };

  # Use a helper function to configure static IP networking.
  systemd.network.networks = util.mkClusterNetworks self;
}
