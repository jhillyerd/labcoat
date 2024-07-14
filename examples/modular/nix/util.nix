{ lib, ... }: {
  # Populate systemd.network.networks given a catalog `self` entry.
  mkClusterNetworks = self: {
    # Hardware config defaults to DHCP, make static if ip.priv is set.
    "10-cluster" = lib.mkIf (self ? ip.priv) {
      networkConfig.DHCP = "no";
      address = [ (self.ip.priv + "/24") ];
      gateway = [ "192.168.1.1" ];

      dns = [
        "192.168.1.1"
        "1.1.1.1"
      ];
      domains = [ "home.arpa" ];
    };
  };
}
