{ system }: {
  # Each entry in this attribute set becomes a specific nixosSystem
  # configuration, aka a node.  Each node is a combination of a
  # host config (`config` attrib) and a hardware config (`hw` attrib.)
  # 
  # The attribute key becomes the node's hostname.
  web1 = {
    ip.priv = "192.168.1.101";
    config = ../hosts/webhost.nix;
    hw = ../hw/proxmox.nix;
    system = system.x86_64-linux;
  };

  web2 = {
    ip.priv = "192.168.1.102";
    config = ../hosts/webhost.nix;
    hw = ../hw/proxmox.nix;
    system = system.x86_64-linux;
  };
}
