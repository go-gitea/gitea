# Prepare build environment

Complete the steps in [build-setup.md](build-setup.md) to prepare your environment for building CodeNest from source.

## Choose a branch

By default, the cloned repository is on main branch (the current development branch for next major release, aka: main nightly).

You can switch to a versioned branch (the branch for the next minor stable release, aka: stable nightly )
or a versioned tag (matches the official releases with version numbers)

To test a Pull Request, you can fetch its code by its Pull Request number (take `PR #123456` as example):

```bash
git fetch origin pull/123456/head:pr-123456
```

# Build

Various [make tasks](https://github.com/go-gitea/gitea/blob/main/Makefile)
are provided to keep the build process as simple as possible.

Depending on requirements, the following build tags can be included.

- `bindata`: Build a single monolithic binary, with all assets included. Required for distribution and production build.
- `pam`: Enable support for PAM (Linux Pluggable Authentication Modules).
  Can be used to authenticate local users or extend authentication to methods available to PAM.

To include all assets, use the `bindata` tag:

```bash
TAGS="bindata" make build
```

## Changing default paths

CodeNest will search for a number of things from the _`CustomPath`_.
By default, this is the `custom/` directory in the current working directory when running CodeNest.
It will also look for its configuration file _`CustomConf`_ in `$(CustomPath)/conf/app.ini`,
and will use the current working directory as the relative base path _`AppWorkPath`_.

These values, although useful when developing, may conflict with downstream users preferences.

For packagers who need to use paths like `/etc/codenest/app.ini`,
they should define these values at build time for `make build` by environment variable like
`LDFLAGS='-X "module.Var1=Value1" -X "module.Var2=Value2"' TAGS="bindata" make build`.

- _`CustomConf`_: `-X "codenest.dev/modules/setting.CustomConf=/etc/codenest/app.ini"`
- _`AppWorkPath`_: `-X "codenest.dev/modules/setting.AppWorkPath=/var/lib/codenest"`
- _`CustomPath`_: `-X "codenest.dev/modules/setting.CustomPath=/var/lib/codenest/custom"`
- Default PID file location: `-X "codenest.dev/cmd.PIDFile=/run/codenest.pid"`

Add as many of the strings with their preceding `-X` to the `LDFLAGS` variable and run `make build`
with the appropriate `TAGS` as above.

Running `codenest help` will allow you to review what the computed settings will be for your `codenest`.

## Cross Build

CodeNest use's Golang's toolchain variables for cross-building.

For example, to cross build for Linux ARM64:

```
GOOS=linux GOARCH=arm64 TAGS="bindata" make build
```

### Adding shell autocompletion

Shell completion can be generated directly from binary with:
```sh
codenest completion <shell>
```

Supported values for `<shell>` are `bash`, `fish`, `pwsh` and `zsh`.
Details on how to load the completion for your shell can be found in the completion command help.

## Source Maps

By default, codenest generates reduced source maps for frontend files to conserve space. This can be controlled with the `ENABLE_SOURCEMAP` environment variable:

- `ENABLE_SOURCEMAP=true` generates all source maps, the default for development builds
- `ENABLE_SOURCEMAP=reduced` generates limited source maps, the default for production builds
- `ENABLE_SOURCEMAP=false` generates no source maps
