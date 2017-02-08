# Goth: Multi-Provider Authentication for Go [![GoDoc](https://godoc.org/github.com/markbates/goth?status.svg)](https://godoc.org/github.com/markbates/goth) [![Build Status](https://travis-ci.org/markbates/goth.svg)](https://travis-ci.org/markbates/goth)

Package goth provides a simple, clean, and idiomatic way to write authentication
packages for Go web applications.

Unlike other similar packages, Goth, lets you write OAuth, OAuth2, or any other
protocol providers, as long as they implement the `Provider` and `Session` interfaces.

This package was inspired by [https://github.com/intridea/omniauth](https://github.com/intridea/omniauth).

## Installation

```text
$ go get github.com/markbates/goth
```

## Supported Providers

* Amazon
* Auth0
* Bitbucket
* Box
* Cloud Foundry
* Dailymotion
* Deezer
* Digital Ocean
* Discord
* Dropbox
* Facebook
* Fitbit
* GitHub
* Gitlab
* Google+
* Heroku
* InfluxCloud
* Instagram
* Intercom
* Lastfm
* Linkedin
* Meetup
* OneDrive
* OpenID Connect (auto discovery)
* Paypal
* SalesForce
* Slack
* Soundcloud
* Spotify
* Steam
* Stripe
* Twitch
* Twitter
* Uber
* Wepay
* Yahoo
* Yammer

## Examples

See the [examples](examples) folder for a working application that lets users authenticate
through Twitter, Facebook, Google Plus etc.

To run the example either clone the source from GitHub

```text
$ git clone git@github.com:markbates/goth.git
```
or use
```text
$ go get github.com/markbates/goth
```
```text
$ cd goth/examples
$ go get -v
$ go build 
$ ./examples
```

Now open up your browser and go to [http://localhost:3000](http://localhost:3000) to see the example.

To actually use the different providers, please make sure you configure them given the system environments as defined in the examples/main.go file

## Issues

Issues always stand a significantly better chance of getting fixed if the are accompanied by a
pull request.

## Contributing

Would I love to see more providers? Certainly! Would you love to contribute one? Hopefully, yes!

1. Fork it
2. Create your feature branch (git checkout -b my-new-feature)
3. Write Tests!
4. Commit your changes (git commit -am 'Add some feature')
5. Push to the branch (git push origin my-new-feature)
6. Create new Pull Request

## Contributors

* Mark Bates
* Tyler Bunnell
* Corey McGrillis
* willemvd
* Rakesh Goyal
* Andy Grunwald
* Glenn Walker
* Kevin Fitzpatrick
* Ben Tranter
* Sharad Ganapathy
* Andrew Chilton
* sharadgana
* Aurorae
* Craig P Jolicoeur
* Zac Bergquist
* Geoff Franks
* Raphael Geronimi
* Noah Shibley
* lumost
* oov
* Felix Lamouroux
* Rafael Quintela
* Tyler
* DenSm
* Samy KACIMI
* dante gray
* Noah
* Jacob Walker
* Marin Martinic
* Roy
* Omni Adams
* Sasa Brankovic
* dkhamsing
* Dante Swift
* Attila Domokos
* Albin Gilles
* Syed Zubairuddin
* Johnny Boursiquot
* Jerome Touffe-Blin
* bryanl
* Masanobu YOSHIOKA
* Jonathan Hall
* HaiMing.Yin
* Sairam Kunala
