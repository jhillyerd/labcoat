# Common config shared among all machines.
{ pkgs
, authorizedKeys
, hostName
, ...
}: {
  system.stateVersion = "24.05";

  imports = [ ./packages.nix ../roles ];
  # nixpkgs.config.allowUnfree = true;

  nix = {
    optimise.automatic = true;

    gc = {
      automatic = true;
      dates = "weekly";
      options = "--delete-older-than 7d";
      randomizedDelaySec = "20min";
    };
  };

  services.openssh = {
    enable = true;
    settings.PermitRootLogin = "yes";
  };
  users.users.root.openssh.authorizedKeys.keys = authorizedKeys;

  programs.command-not-found.enable = false; # not flake aware

  time.timeZone = "US/Pacific";

  # Display node name and IP address above login prompt.
  services.getty.helpLine = ">>> Flake node: ${hostName}";
  environment.etc."issue.d/ip.issue".text = ''
    IPv4: \4
  '';
  networking.dhcpcd.runHook = "${pkgs.utillinux}/bin/agetty --reload";
}
