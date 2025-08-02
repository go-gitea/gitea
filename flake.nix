{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };
  outputs =
    { nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default =
          with pkgs;
          let
            # only bump toolchain versions here
            go = go_1_24;
            nodejs = nodejs_24;
            python3 = python312;
          in
          pkgs.mkShell {
            buildInputs = [
              # generic
              git
              git-lfs
              gnumake
              gnused
              gnutar
              gzip

              # frontend
              nodejs

              # linting
              python3
              uv

              # backend
              go
              glibc.static
              gofumpt
              sqlite
            ];
            CFLAGS = "-I${glibc.static.dev}/include";
            LDFLAGS = "-L ${glibc.static}/lib";
            GO = "${go}/bin/go";
            GOROOT = "${go}/share/go";

            TAGS = "sqlite sqlite_unlock_notify";
            STATIC = "true";
          };
      }
    );
}
