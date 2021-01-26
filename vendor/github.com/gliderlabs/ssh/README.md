# gliderlabs/ssh

[![GoDoc](https://godoc.org/github.com/gliderlabs/ssh?status.svg)](https://godoc.org/github.com/gliderlabs/ssh) 
[![CircleCI](https://img.shields.io/circleci/project/github/gliderlabs/ssh.svg)](https://circleci.com/gh/gliderlabs/ssh)
[![Go Report Card](https://goreportcard.com/badge/github.com/gliderlabs/ssh)](https://goreportcard.com/report/github.com/gliderlabs/ssh) 
[![OpenCollective](https://opencollective.com/ssh/sponsors/badge.svg)](#sponsors)
[![Slack](http://slack.gliderlabs.com/badge.svg)](http://slack.gliderlabs.com) 
[![Email Updates](https://img.shields.io/badge/updates-subscribe-yellow.svg)](https://app.convertkit.com/landing_pages/243312)

> The Glider Labs SSH server package is dope.  &mdash;[@bradfitz](https://twitter.com/bradfitz), Go team member

This Go package wraps the [crypto/ssh
package](https://godoc.org/golang.org/x/crypto/ssh) with a higher-level API for
building SSH servers. The goal of the API was to make it as simple as using
[net/http](https://golang.org/pkg/net/http/), so the API is very similar:

```go
 package main

 import (
     "github.com/gliderlabs/ssh"
     "io"
     "log"
 )

 func main() {
     ssh.Handle(func(s ssh.Session) {
         io.WriteString(s, "Hello world\n")
     })  

     log.Fatal(ssh.ListenAndServe(":2222", nil))
 }

```
This package was built by [@progrium](https://twitter.com/progrium) after working on nearly a dozen projects at Glider Labs using SSH and collaborating with [@shazow](https://twitter.com/shazow) (known for [ssh-chat](https://github.com/shazow/ssh-chat)).

## Examples

A bunch of great examples are in the `_examples` directory.

## Usage

[See GoDoc reference.](https://godoc.org/github.com/gliderlabs/ssh)

## Contributing

Pull requests are welcome! However, since this project is very much about API
design, please submit API changes as issues to discuss before submitting PRs.

Also, you can [join our Slack](http://slack.gliderlabs.com) to discuss as well.

## Roadmap

* Non-session channel handlers
* Cleanup callback API
* 1.0 release
* High-level client?

## Sponsors

Become a sponsor and get your logo on our README on Github with a link to your site. [[Become a sponsor](https://opencollective.com/ssh#sponsor)]

<a href="https://opencollective.com/ssh/sponsor/0/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/0/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/1/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/1/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/2/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/2/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/3/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/3/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/4/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/4/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/5/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/5/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/6/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/6/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/7/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/7/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/8/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/8/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/9/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/9/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/10/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/10/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/11/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/11/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/12/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/12/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/13/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/13/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/14/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/14/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/15/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/15/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/16/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/16/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/17/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/17/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/18/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/18/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/19/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/19/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/20/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/20/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/21/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/21/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/22/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/22/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/23/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/23/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/24/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/24/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/25/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/25/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/26/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/26/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/27/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/27/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/28/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/28/avatar.svg"></a>
<a href="https://opencollective.com/ssh/sponsor/29/website" target="_blank"><img src="https://opencollective.com/ssh/sponsor/29/avatar.svg"></a>

## License

[BSD](LICENSE)
