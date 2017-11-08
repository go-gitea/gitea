# Gitea: Docs

[![Build Status](http://drone.gitea.io/api/badges/go-gitea/docs/status.svg)](http://drone.gitea.io/go-gitea/docs)
[![Join the chat at https://img.shields.io/discord/322538954119184384.svg](https://img.shields.io/discord/322538954119184384.svg)](https://discord.gg/NsatcWJ)
[![](https://images.microbadger.com/badges/image/gitea/docs.svg)](http://microbadger.com/images/gitea/docs "Get your own image badge on microbadger.com")

## Hosting

This page is hosted on our infrastructure within Docker containers, it gets
automatically updated on every push to the `master` branch.

If you want to host this page on your own you can take our docker image
[gitea/docs](https://hub.docker.com/r/gitea/docs/).

## Install

This pages uses the [Hugo](https://github.com/spf13/hugo) static site generator.
If you are planning to contribute you'll want to download and install Hugo on
your local machine.

The installation of Hugo is out of the scope of this document, so please take
the [official install instructions](https://gohugo.io/overview/installing/) to
get Hugo up and running.

## Development

To generate the website and serve it on [localhost:1313](http://localhost:1313)
just execute this command and stop it with `Ctrl+C`:

```
make server
```

When you are done with your changes just create a pull request, after merging
the pull request the website will be updated automatically.

## Contributing

Fork -> Patch -> Push -> Pull Request

## Authors

* [Maintainers](https://github.com/orgs/go-gitea/people)
* [Contributors](https://github.com/go-gitea/docs/graphs/contributors)

## License

This project is under the Apache-2.0 License. See the [LICENSE](LICENSE) file
for the full license text.

## Copyright

```
Copyright (c) 2016 The Gitea Authors <https://gitea.io>
```
