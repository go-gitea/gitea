{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };
  outputs =
    { nixpkgs, ... }:
    let
      supportedSystems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-darwin"
        "x86_64-linux"
      ];

      forEachSupportedSystem =
        f:
        nixpkgs.lib.genAttrs supportedSystems (
          system:
          let
            pkgs = import nixpkgs {
              inherit system;
            };
          in
          f { inherit pkgs; }
        );
    in
    {
      devShells = forEachSupportedSystem (
        { pkgs, ... }:
        {
          default =
            let
              inherit (pkgs) lib;

              # only bump toolchain versions here
              go = pkgs.go_1_25;
              nodejs = pkgs.nodejs_24;
              python3 = pkgs.python312;
              pnpm = pkgs.pnpm_10;

              # Platform-specific dependencies
              linuxOnlyInputs = lib.optionals pkgs.stdenv.isLinux [
                pkgs.glibc.static
              ];

              linuxOnlyEnv = lib.optionalAttrs pkgs.stdenv.isLinux {
                CFLAGS = "-I${pkgs.glibc.static.dev}/include";
                LDFLAGS = "-L ${pkgs.glibc.static}/lib";
              };
            in
            pkgs.mkShell {
              packages =
                with pkgs;
                [
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

              env = {
                GO = "${go}/bin/go";
                GOROOT = "${go}/share/go";

                TAGS = "sqlite sqlite_unlock_notify";
                STATIC = "true";
              }
              // linuxOnlyEnv;
            };
        }
      );
    };
}
