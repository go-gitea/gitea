{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-23.11";
  };
  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # generic
            git
            git-lfs
            gnumake
            gnutar

            # frontend
            nodejs_20
            nodePackages.npm

            # backend
            go_1_20
            gofumpt
            golangci-lint
          ];
        };
      }
    );
}
