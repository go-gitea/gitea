<p align="center">
	<a href="https://pkg.go.dev/github.com/caddyserver/certmagic?tab=doc"><img src="https://user-images.githubusercontent.com/1128849/49704830-49d37200-fbd5-11e8-8385-767e0cd033c3.png" alt="CertMagic" width="550"></a>
</p>
<h3 align="center">Easy and Powerful TLS Automation</h3>
<p align="center">The same library used by the <a href="https://caddyserver.com">Caddy Web Server</a></p>
<p align="center">
	<a href="https://pkg.go.dev/github.com/caddyserver/certmagic?tab=doc"><img src="https://img.shields.io/badge/godoc-reference-blue.svg"></a>
	<a href="https://github.com/caddyserver/certmagic/actions?query=workflow%3ATests"><img src="https://github.com/caddyserver/certmagic/workflows/Tests/badge.svg"></a>
	<a href="https://sourcegraph.com/github.com/caddyserver/certmagic?badge"><img src="https://sourcegraph.com/github.com/caddyserver/certmagic/-/badge.svg"></a>
</p>


Caddy's automagic TLS features&mdash;now for your own Go programs&mdash;in one powerful and easy-to-use library!

CertMagic is the most mature, robust, and capable ACME client integration for Go... and perhaps ever.

With CertMagic, you can add one line to your Go application to serve securely over TLS, without ever having to touch certificates.

Instead of:

```go
// plaintext HTTP, gross 🤢
http.ListenAndServe(":80", mux)
```

Use CertMagic:

```go
// encrypted HTTPS with HTTP->HTTPS redirects - yay! 🔒😍
certmagic.HTTPS([]string{"example.com"}, mux)
```

That line of code will serve your HTTP router `mux` over HTTPS, complete with HTTP->HTTPS redirects. It obtains and renews the TLS certificates. It staples OCSP responses for greater privacy and security. As long as your domain name points to your server, CertMagic will keep its connections secure.

Compared to other ACME client libraries for Go, only CertMagic supports the full suite of ACME features, and no other library matches CertMagic's maturity and reliability.




CertMagic - Automatic HTTPS using Let's Encrypt
===============================================

**Sponsored by Relica - Cross-platform local and cloud file backup:**

<a href="https://relicabackup.com"><img src="https://caddyserver.com/resources/images/sponsors/relica.png" width="220" alt="Relica - Cross-platform file backup to the cloud, local disks, or other computers"></a>


## Menu

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
	- [Package Overview](#package-overview)
		- [Certificate authority](#certificate-authority)
		- [The `Config` type](#the-config-type)
		- [Defaults](#defaults)
		- [Providing an email address](#providing-an-email-address)
		- [Rate limiting](#rate-limiting)
	- [Development and testing](#development-and-testing)
	- [Examples](#examples)
		- [Serving HTTP handlers with HTTPS](#serving-http-handlers-with-https)
		- [Starting a TLS listener](#starting-a-tls-listener)
		- [Getting a tls.Config](#getting-a-tlsconfig)
		- [Advanced use](#advanced-use)
	- [Wildcard Certificates](#wildcard-certificates)
	- [Behind a load balancer (or in a cluster)](#behind-a-load-balancer-or-in-a-cluster)
	- [The ACME Challenges](#the-acme-challenges)
		- [HTTP Challenge](#http-challenge)
		- [TLS-ALPN Challenge](#tls-alpn-challenge)
		- [DNS Challenge](#dns-challenge)
	- [On-Demand TLS](#on-demand-tls)
	- [Storage](#storage)
	- [Cache](#cache)
- [Contributing](#contributing)
- [Project History](#project-history)
- [Credits and License](#credits-and-license)


## Features

- Fully automated certificate management including issuance and renewal
- One-liner, fully managed HTTPS servers
- Full control over almost every aspect of the system
- HTTP->HTTPS redirects
- Solves all 3 ACME challenges: HTTP, TLS-ALPN, and DNS
- Most robust error handling of _any_ ACME client
	- Challenges are randomized to avoid accidental dependence
	- Challenges are rotated to overcome certain network blockages
	- Robust retries for up to 30 days
	- Exponential backoff with carefully-tuned intervals
	- Retries with optional test/staging CA endpoint instead of production, to avoid rate limits
- Written in Go, a language with memory-safety guarantees
- Powered by [ACMEz](https://github.com/mholt/acmez), _the_ premier ACME client library for Go
- All [libdns](https://github.com/libdns) DNS providers work out-of-the-box
- Pluggable storage implementations (default: file system)
- Wildcard certificates
- Automatic OCSP stapling ([done right](https://gist.github.com/sleevi/5efe9ef98961ecfb4da8#gistcomment-2336055)) [keeps your sites online!](https://twitter.com/caddyserver/status/1234874273724084226)
	- Will [automatically attempt](https://twitter.com/mholt6/status/1235577699541762048) to replace [revoked certificates](https://community.letsencrypt.org/t/2020-02-29-caa-rechecking-bug/114591/3?u=mholt)!
	- Staples stored to disk in case of responder outages
- Distributed solving of all challenges (works behind load balancers)
	- Highly efficient, coordinated management in a fleet
	- Active locking
	- Smart queueing
- Supports "on-demand" issuance of certificates (during TLS handshakes!)
	- Caddy / CertMagic pioneered this technology
	- Custom decision functions to regulate and throttle on-demand behavior
- Optional event hooks for observation
- Works with any certificate authority (CA) compliant with the ACME specification
- Certificate revocation (please, only if private key is compromised)
- Must-Staple (optional; not default)
- Cross-platform support! Mac, Windows, Linux, BSD, Android...
- Scales to hundreds of thousands of names/certificates per instance
- Use in conjunction with your own certificates


## Requirements

1. Public DNS name(s) you control
2. Server reachable from public Internet
	- Or use the DNS challenge to waive this requirement
3. Control over port 80 (HTTP) and/or 443 (HTTPS)
	- Or they can be forwarded to other ports you control
	- Or use the DNS challenge to waive this requirement
	- (This is a requirement of the ACME protocol, not a library limitation)
4. Persistent storage
	- Typically the local file system (default)
	- Other integrations available/possible

**_Before using this library, your domain names MUST be pointed (A/AAAA records) at your server (unless you use the DNS challenge)!_**


## Installation

```bash
$ go get github.com/caddyserver/certmagic
```


## Usage

### Package Overview

#### Certificate authority

This library uses Let's Encrypt by default, but you can use any certificate authority that conforms to the ACME specification. Known/common CAs are provided as consts in the package, for example `LetsEncryptStagingCA` and `LetsEncryptProductionCA`.

#### The `Config` type

The `certmagic.Config` struct is how you can wield the power of this fully armed and operational battle station. However, an empty/uninitialized `Config` is _not_ a valid one! In time, you will learn to use the force of `certmagic.NewDefault()` as I have.

#### Defaults

The default `Config` value is called `certmagic.Default`. Change its fields to suit your needs, then call `certmagic.NewDefault()` when you need a valid `Config` value. In other words, `certmagic.Default` is a template and is not valid for use directly.

You can set the default values easily, for example: `certmagic.Default.Issuer = ...`.

Similarly, to configure ACME-specific defaults, use `certmagic.DefaultACME`.

The high-level functions in this package (`HTTPS()`, `Listen()`, `ManageSync()`, and `ManageAsync()`) use the default config exclusively. This is how most of you will interact with the package. This is suitable when all your certificates are managed the same way. However, if you need to manage certificates differently depending on their name, you will need to make your own cache and configs (keep reading).


#### Providing an email address

Although not strictly required, this is highly recommended best practice. It allows you to receive expiration emails if your certificates are expiring for some reason, and also allows the CA's engineers to potentially get in touch with you if something is wrong. I recommend setting `certmagic.DefaultACME.Email` or always setting the `Email` field of a new `Config` struct.


#### Rate limiting

To avoid firehosing the CA's servers, CertMagic has built-in rate limiting. Currently, its default limit is up to 10 transactions (obtain or renew) every 1 minute (sliding window). This can be changed by setting the `RateLimitEvents` and `RateLimitEventsWindow` variables, if desired.

The CA may still enforce their own rate limits, and there's nothing (well, nothing ethical) CertMagic can do to bypass them for you.

Additionally, CertMagic will retry failed validations with exponential backoff for up to 30 days, with a reasonable maximum interval between attempts (an "attempt" means trying each enabled challenge type once).


### Development and Testing

Note that Let's Encrypt imposes [strict rate limits](https://letsencrypt.org/docs/rate-limits/) at its production endpoint, so using it while developing your application may lock you out for a few days if you aren't careful!

While developing your application and testing it, use [their staging endpoint](https://letsencrypt.org/docs/staging-environment/) which has much higher rate limits. Even then, don't hammer it: but it's much safer for when you're testing. When deploying, though, use their production CA because their staging CA doesn't issue trusted certificates.

To use staging, set `certmagic.DefaultACME.CA = certmagic.LetsEncryptStagingCA` or set `CA` of every `ACMEManager` struct.



### Examples

There are many ways to use this library. We'll start with the highest-level (simplest) and work down (more control).

All these high-level examples use `certmagic.Default` and `certmagic.DefaultACME` for the config and the default cache and storage for serving up certificates.

First, we'll follow best practices and do the following:

```go
// read and agree to your CA's legal documents
certmagic.DefaultACME.Agreed = true

// provide an email address
certmagic.DefaultACME.Email = "you@yours.com"

// use the staging endpoint while we're developing
certmagic.DefaultACME.CA = certmagic.LetsEncryptStagingCA
```

For fully-functional program examples, check out [this Twitter thread](https://twitter.com/mholt6/status/1073103805112147968) (or read it [unrolled into a single post](https://threadreaderapp.com/thread/1073103805112147968.html)). (Note that the package API has changed slightly since these posts.)


#### Serving HTTP handlers with HTTPS

```go
err := certmagic.HTTPS([]string{"example.com", "www.example.com"}, mux)
if err != nil {
	return err
}
```

This starts HTTP and HTTPS listeners and redirects HTTP to HTTPS!

#### Starting a TLS listener

```go
ln, err := certmagic.Listen([]string{"example.com"})
if err != nil {
	return err
}
```


#### Getting a tls.Config

```go
tlsConfig, err := certmagic.TLS([]string{"example.com"})
if err != nil {
	return err
}
```


#### Advanced use

For more control (particularly, if you need a different way of managing each certificate), you'll make and use a `Cache` and a `Config` like so:

```go
cache := certmagic.NewCache(certmagic.CacheOptions{
	GetConfigForCert: func(cert certmagic.Certificate) (*certmagic.Config, error) {
		// do whatever you need to do to get the right
		// configuration for this certificate; keep in
		// mind that this config value is used as a
		// template, and will be completed with any
		// defaults that are set in the Default config
		return &certmagic.Config{
			// ...
		}, nil
	},
	...
})

magic := certmagic.New(cache, certmagic.Config{
	// any customizations you need go here
})

myACME := certmagic.NewACMEManager(magic, certmagic.ACMEManager{
	CA:     certmagic.LetsEncryptStagingCA,
	Email:  "you@yours.com",
	Agreed: true,
	// plus any other customizations you need
})

magic.Issuer = myACME

// this obtains certificates or renews them if necessary
err := magic.ManageSync([]string{"example.com", "sub.example.com"})
if err != nil {
	return err
}

// to use its certificates and solve the TLS-ALPN challenge,
// you can get a TLS config to use in a TLS listener!
tlsConfig := magic.TLSConfig()

//// OR ////

// if you already have a TLS config you don't want to replace,
// we can simply set its GetCertificate field and append the
// TLS-ALPN challenge protocol to the NextProtos
myTLSConfig.GetCertificate = magic.GetCertificate
myTLSConfig.NextProtos = append(myTLSConfig.NextProtos, tlsalpn01.ACMETLS1Protocol)

// the HTTP challenge has to be handled by your HTTP server;
// if you don't have one, you should have disabled it earlier
// when you made the certmagic.Config
httpMux = myACME.HTTPChallengeHandler(httpMux)
```

Great! This example grants you much more flexibility for advanced programs. However, _the vast majority of you will only use the high-level functions described earlier_, especially since you can still customize them by setting the package-level `Default` config.


### Wildcard certificates

At time of writing (December 2018), Let's Encrypt only issues wildcard certificates with the DNS challenge. You can easily enable the DNS challenge with CertMagic for numerous providers (see the relevant section in the docs).


### Behind a load balancer (or in a cluster)

CertMagic runs effectively behind load balancers and/or in cluster/fleet environments. In other words, you can have 10 or 1,000 servers all serving the same domain names, all sharing certificates and OCSP staples.

To do so, simply ensure that each instance is using the same Storage. That is the sole criteria for determining whether an instance is part of a cluster.

The default Storage is implemented using the file system, so mounting the same shared folder is sufficient (see [Storage](#storage) for more on that)! If you need an alternate Storage implementation, feel free to use one, provided that all the instances use the _same_ one. :)

See [Storage](#storage) and the associated [pkg.go.dev](https://pkg.go.dev/github.com/caddyserver/certmagic?tab=doc#Storage) for more information!


## The ACME Challenges

This section describes how to solve the ACME challenges. Challenges are how you demonstrate to the certificate authority some control over your domain name, thus authorizing them to grant you a certificate for that name. [The great innovation of ACME](https://www.dotconferences.com/2016/10/matthew-holt-go-with-acme) is that verification by CAs can now be automated, rather than having to click links in emails (who ever thought that was a good idea??).

If you're using the high-level convenience functions like `HTTPS()`, `Listen()`, or `TLS()`, the HTTP and/or TLS-ALPN challenges are solved for you because they also start listeners. However, if you're making a `Config` and you start your own server manually, you'll need to be sure the ACME challenges can be solved so certificates can be renewed.

The HTTP and TLS-ALPN challenges are the defaults because they don't require configuration from you, but they require that your server is accessible from external IPs on low ports. If that is not possible in your situation, you can enable the DNS challenge, which will disable the HTTP and TLS-ALPN challenges and use the DNS challenge exclusively.

Technically, only one challenge needs to be enabled for things to work, but using multiple is good for reliability in case a challenge is discontinued by the CA. This happened to the TLS-SNI challenge in early 2018&mdash;many popular ACME clients such as Traefik and Autocert broke, resulting in downtime for some sites, until new releases were made and patches deployed, because they used only one challenge; Caddy, however&mdash;this library's forerunner&mdash;was unaffected because it also used the HTTP challenge. If multiple challenges are enabled, they are chosen randomly to help prevent false reliance on a single challenge type. And if one fails, any remaining enabled challenges are tried before giving up.


### HTTP Challenge

Per the ACME spec, the HTTP challenge requires port 80, or at least packet forwarding from port 80. It works by serving a specific HTTP response that only the genuine server would have to a normal HTTP request at a special endpoint.

If you are running an HTTP server, solving this challenge is very easy: just wrap your handler in `HTTPChallengeHandler` _or_ call `SolveHTTPChallenge()` inside your own `ServeHTTP()` method.

For example, if you're using the standard library:

```go
mux := http.NewServeMux()
mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "Lookit my cool website over HTTPS!")
})

http.ListenAndServe(":80", myACME.HTTPChallengeHandler(mux))
```

If wrapping your handler is not a good solution, try this inside your `ServeHTTP()` instead:

```go
magic := certmagic.NewDefault()
myACME := certmagic.NewACMEManager(magic, certmagic.DefaultACME)

func ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if myACME.HandleHTTPChallenge(w, r) {
		return // challenge handled; nothing else to do
	}
	...
}
```

If you are not running an HTTP server, you should disable the HTTP challenge _or_ run an HTTP server whose sole job it is to solve the HTTP challenge.


### TLS-ALPN Challenge

Per the ACME spec, the TLS-ALPN challenge requires port 443, or at least packet forwarding from port 443. It works by providing a special certificate using a standard TLS extension, Application Layer Protocol Negotiation (ALPN), having a special value. This is the most convenient challenge type because it usually requires no extra configuration and uses the standard TLS port which is where the certificates are used, also.

This challenge is easy to solve: just use the provided `tls.Config` when you make your TLS listener:

```go
// use this to configure a TLS listener
tlsConfig := magic.TLSConfig()
```

Or make two simple changes to an existing `tls.Config`:

```go
myTLSConfig.GetCertificate = magic.GetCertificate
myTLSConfig.NextProtos = append(myTLSConfig.NextProtos, tlsalpn01.ACMETLS1Protocol}
```

Then just make sure your TLS listener is listening on port 443:

```go
ln, err := tls.Listen("tcp", ":443", myTLSConfig)
```


### DNS Challenge

The DNS challenge is perhaps the most useful challenge because it allows you to obtain certificates without your server needing to be publicly accessible on the Internet, and it's the only challenge by which Let's Encrypt will issue wildcard certificates.

This challenge works by setting a special record in the domain's zone. To do this automatically, your DNS provider needs to offer an API by which changes can be made to domain names, and the changes need to take effect immediately for best results. CertMagic supports [all DNS providers with `libdns` implementations](https://github.com/libdns)! It always cleans up the temporary record after the challenge completes.

To enable it, just set the `DNS01Solver` field on a `certmagic.ACMEManager` struct, or set the default `certmagic.ACMEManager.DNS01Solver` variable. For example, if my domains' DNS was served by Cloudflare:

```go
import "github.com/libdns/cloudflare"

certmagic.DefaultACME.DNS01Solver = &certmagic.DNS01Solver{
	DNSProvider: &cloudflare.Provider{
		APIToken: "topsecret",
	},
}
```

Now the DNS challenge will be used by default, and I can obtain certificates for wildcard domains, too. Enabling the DNS challenge disables the other challenges for that `certmagic.ACMEManager` instance.


## On-Demand TLS

Normally, certificates are obtained and renewed before a listener starts serving, and then those certificates are maintained throughout the lifetime of the program. In other words, the certificate names are static. But sometimes you don't know all the names ahead of time, or you don't want to manage all the certificates up front. This is where On-Demand TLS shines.

Originally invented for use in Caddy (which was the first program to use such technology), On-Demand TLS makes it possible and easy to serve certificates for arbitrary or specific names during the lifetime of the server. When a TLS handshake is received, CertMagic will read the Server Name Indication (SNI) value and either load and present that certificate in the ServerHello, or if one does not exist, it will obtain it from a CA right then-and-there.

Of course, this has some obvious security implications. You don't want to DoS a CA or allow arbitrary clients to fill your storage with spammy TLS handshakes. That's why, when you enable On-Demand issuance, you should set limits or policy to allow getting certificates. CertMagic has an implicit whitelist built-in which is sufficient for nearly everyone, but also has a more advanced way to control on-demand issuance.

The simplest way to enable on-demand issuance is to set the OnDemand field of a Config (or the default package-level value):

```go
certmagic.Default.OnDemand = new(certmagic.OnDemandConfig)
```

By setting this to a non-nil value, on-demand TLS is enabled for that config. For convenient security, CertMagic's high-level abstraction functions such as `HTTPS()`, `TLS()`, `ManageSync()`, `ManageAsync()`, and `Listen()` (which all accept a list of domain names) will whitelist those names automatically so only certificates for those names can be obtained when using the Default config. Usually this is sufficient for most users.

However, if you require advanced control over which domains can be issued certificates on-demand (for example, if you do not know which domain names you are managing, or just need to defer their operations until later), you should implement your own DecisionFunc:

```go
// if the decision function returns an error, a certificate
// may not be obtained for that name at that time
certmagic.Default.OnDemand = &certmagic.OnDemandConfig{
	DecisionFunc: func(name string) error {
		if name != "example.com" {
			return fmt.Errorf("not allowed")
		}
		return nil
	},
}
```

The [pkg.go.dev](https://pkg.go.dev/github.com/caddyserver/certmagic?tab=doc#OnDemandConfig) describes how to use this in full detail, so please check it out!


## Storage

CertMagic relies on storage to store certificates and other TLS assets (OCSP staple cache, coordinating locks, etc). Persistent storage is a requirement when using CertMagic: ephemeral storage will likely lead to rate limiting on the CA-side as CertMagic will always have to get new certificates.

By default, CertMagic stores assets on the local file system in `$HOME/.local/share/certmagic` (and honors `$XDG_DATA_HOME` if set). CertMagic will create the directory if it does not exist. If writes are denied, things will not be happy, so make sure CertMagic can write to it!

The notion of a "cluster" or "fleet" of instances that may be serving the same site and sharing certificates, etc, is tied to storage. Simply, any instances that use the same storage facilities are considered part of the cluster. So if you deploy 100 instances of CertMagic behind a load balancer, they are all part of the same cluster if they share the same storage configuration. Sharing storage could be mounting a shared folder, or implementing some other distributed storage system such as a database server or KV store.

The easiest way to change the storage being used is to set `certmagic.DefaultStorage` to a value that satisfies the [Storage interface](https://pkg.go.dev/github.com/caddyserver/certmagic?tab=doc#Storage). Keep in mind that a valid `Storage` must be able to implement some operations atomically in order to provide locking and synchronization.

If you write a Storage implementation, please add it to the [project wiki](https://github.com/caddyserver/certmagic/wiki/Storage-Implementations) so people can find it!


## Cache

All of the certificates in use are de-duplicated and cached in memory for optimal performance at handshake-time. This cache must be backed by persistent storage as described above.

Most applications will not need to interact with certificate caches directly. Usually, the closest you will come is to set the package-wide `certmagic.DefaultStorage` variable (before attempting to create any Configs). However, if your use case requires using different storage facilities for different Configs (that's highly unlikely and NOT recommended! Even Caddy doesn't get that crazy), you will need to call `certmagic.NewCache()` and pass in the storage you want to use, then get new `Config` structs with `certmagic.NewWithCache()` and pass in the cache.

Again, if you're needing to do this, you've probably over-complicated your application design.


## FAQ

### Can I use some of my own certificates while using CertMagic?

Yes, just call the relevant method on the `Config` to add your own certificate to the cache:

- [`CacheUnmanagedCertificatePEMBytes()`](https://pkg.go.dev/github.com/caddyserver/certmagic?tab=doc#Config.CacheUnmanagedCertificatePEMBytes)
- [`CacheUnmanagedCertificatePEMFile()`](https://pkg.go.dev/github.com/caddyserver/certmagic?tab=doc#Config.CacheUnmanagedCertificatePEMFile)
- [`CacheUnmanagedTLSCertificate()`](https://pkg.go.dev/github.com/caddyserver/certmagic?tab=doc#Config.CacheUnmanagedTLSCertificate)

Keep in mind that unmanaged certificates are (obviously) not renewed for you, so you'll have to replace them when you do. However, OCSP stapling is performed even for unmanaged certificates that qualify.


### Does CertMagic obtain SAN certificates?

Technically all certificates these days are SAN certificates because CommonName is deprecated. But if you're asking whether CertMagic issues and manages certificates with multiple SANs, the answer is no. But it does support serving them, if you provide your own.


### How can I listen on ports 80 and 443? Do I have to run as root?

On Linux, you can use `setcap` to grant your binary the permission to bind low ports:

```bash
$ sudo setcap cap_net_bind_service=+ep /path/to/your/binary
```

and then you will not need to run with root privileges.


## Contributing

We welcome your contributions! Please see our **[contributing guidelines](https://github.com/caddyserver/certmagic/blob/master/.github/CONTRIBUTING.md)** for instructions.


## Project History

CertMagic is the core of Caddy's advanced TLS automation code, extracted into a library. The underlying ACME client implementation is [ACMEz](https://github.com/mholt/acmez). CertMagic's code was originally a central part of Caddy even before Let's Encrypt entered public beta in 2015.

In the years since then, Caddy's TLS automation techniques have been widely adopted, tried and tested in production, and served millions of sites and secured trillions of connections.

Now, CertMagic is _the actual library used by Caddy_. It's incredibly powerful and feature-rich, but also easy to use for simple Go programs: one line of code can enable fully-automated HTTPS applications with HTTP->HTTPS redirects.

Caddy is known for its robust HTTPS+ACME features. When ACME certificate authorities have had outages, in some cases Caddy was the only major client that didn't experience any downtime. Caddy can weather OCSP outages lasting days, or CA outages lasting weeks, without taking your sites offline.

Caddy was also the first to sport "on-demand" issuance technology, which obtains certificates during the first TLS handshake for an allowed SNI name.

Consequently, CertMagic brings all these (and more) features and capabilities right into your own Go programs.

You can [watch a 2016 dotGo talk](https://www.dotconferences.com/2016/10/matthew-holt-go-with-acme) by the author of this library about using ACME to automate certificate management in Go programs:

[![Matthew Holt speaking at dotGo 2016 about ACME in Go](https://user-images.githubusercontent.com/1128849/49921557-2d506780-fe6b-11e8-97bf-6053b6b4eb48.png)](https://www.dotconferences.com/2016/10/matthew-holt-go-with-acme)



## Credits and License

CertMagic is a project by [Matthew Holt](https://twitter.com/mholt6), who is the author; and various contributors, who are credited in the commit history of either CertMagic or Caddy.

CertMagic is licensed under Apache 2.0, an open source license. For convenience, its main points are summarized as follows (but this is no replacement for the actual license text):

- The author owns the copyright to this code
- Use, distribute, and modify the software freely
- Private and internal use is allowed
- License text and copyright notices must stay intact and be included with distributions
- Any and all changes to the code must be documented
