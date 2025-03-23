{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";

    flake-parts.url = "github:hercules-ci/flake-parts";
    flake-parts.inputs = {
      nixpkgs-lib.follows = "nixpkgs";
    };
  };

  outputs = inputs@{ self, flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [ "x86_64-linux" "aarch64-linux" ];

      perSystem = { self', pkgs, ... }:
        let
          # Generate a user-friendly version number.
          version = builtins.substring 0 8 self.lastModifiedDate;
        in
        {
          packages.labcoat = pkgs.buildGoModule {
            pname = "labcoat";
            inherit version;
            src = ./.;

            # Must be updated if go.mod changes.
            vendorHash = "sha256-FU+oKZIOfoGWONIRq1rL9T2WYK0P2Rq1ZKL6LNE4zpo=";

            meta.mainProgram = "labcoat";
          };

          packages.default = self'.packages.labcoat;

          devShells.default = pkgs.mkShell {
            buildInputs = with pkgs; [
              delve
              go_1_23
              golint
              gopls
              vhs
            ];

            hardeningDisable = [ "fortify" ];
          };
        };
    };
}
