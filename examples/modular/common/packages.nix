# Packages common to all nodes.
{ pkgs, nixpkgs-unstable, ... }: {
  environment.systemPackages =
    let
      unstable = nixpkgs-unstable.legacyPackages.${pkgs.system};
    in
    (with pkgs; [
      bind
      file
      git
      htop
      jq
      lsof
      mailutils
      nmap
      psmisc
      wget
    ]) ++ (with unstable; [
      neovim
    ]);
}
