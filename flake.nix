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
            go = go_1_25;
            nodejs = nodejs_24;
            python3 = python312;
            pnpm = pnpm_10;

            # Platform-specific dependencies
            linuxOnlyInputs = lib.optionals pkgs.stdenv.isLinux [
              glibc.static
            ];

            linuxOnlyEnv = lib.optionalAttrs pkgs.stdenv.isLinux {
              CFLAGS = "-I${glibc.static.dev}/include";
              LDFLAGS = "-L ${glibc.static}/lib";
            };
          in
          pkgs.mkShell (
            {
              buildInputs = [
                # generic
                git
                git-lfs
                gnumake
                gnused
                gnutar
                gzip
                zip

                # frontend
                nodejs
                pnpm
                cairo
                pixman
                pkg-config

                # linting
                python3
                uv

                # backend
                go
                gofumpt
                sqlite
              ]
              ++ linuxOnlyInputs;

              GO = "${go}/bin/go";
              GOROOT = "${go}/share/go";

              TAGS = "sqlite sqlite_unlock_notify";
              STATIC = "true";
            }
            // linuxOnlyEnv
          );
      }
    );
}
