acmez - ACME client library for Go
==================================

[![godoc](https://pkg.go.dev/badge/github.com/mholt/acmez)](https://pkg.go.dev/github.com/mholt/acmez)

ACMEz ("ack-measy" or "acme-zee", whichever you prefer) is a fully-compliant [RFC 8555](https://tools.ietf.org/html/rfc8555) (ACME) implementation in pure Go. It is lightweight, has an elegant Go API, and its retry logic is highly robust against external errors. ACMEz is suitable for large-scale enterprise deployments.

**NOTE:** This module is for _getting_ certificates, not _managing_ certificates. Most users probably want certificate _management_ (keeping certificates renewed) rather than to interface directly with ACME. Developers who want to use certificates in their long-running Go programs should use [CertMagic](https://github.com/caddyserver/certmagic) instead; or, if their program is not written in Go, [Caddy](https://caddyserver.com/) can be used to manage certificates (even without running an HTTP or TLS server).

This module has two primary packages:

- **`acmez`** is a high-level wrapper for getting certificates. It implements the ACME order flow described in RFC 8555 including challenge solving using pluggable solvers.
- **`acme`** is a low-level RFC 8555 implementation that provides the fundamental ACME operations, mainly useful if you have advanced or niche requirements.

In other words, the `acmez` package is **porcelain** while the `acme` package is **plumbing** (to use git's terminology).


## Features

- Simple, elegant Go API
- Thoroughly documented with spec citations
- Robust to external errors
- Structured error values ("problems" as defined in RFC 7807)
- Smart retries (resilient against network and server hiccups)
- Challenge plasticity (randomized challenges, and will retry others if one fails)
- Context cancellation (suitable for high-frequency config changes or reloads)
- Highly flexible and customizable
- External Account Binding (EAB) support
- Tested with multiple ACME CAs (more than just Let's Encrypt)
- Supports niche aspects of RFC 8555 (such as alt cert chains and account key rollover)
- Efficient solving of large SAN lists (e.g. for slow DNS record propagation)
- Utility functions for solving challenges
- Helpers for RFC 8737 (tls-alpn-01 challenge)

The `acmez` package is "bring-your-own-solver." It provides helper utilities for http-01, dns-01, and tls-alpn-01 challenges, but does not actually solve them for you. You must write an implementation of `acmez.Solver` in order to get certificates. How this is done depends on the environment in which you're using this code.

This is not a command line utility either. The goal is to not add more external tooling to already-complex infrastructure: ACME and TLS should be built-in to servers rather than tacked on as an afterthought.


## Examples

See the `examples` folder for tutorials on how to use either package.


## History

In 2014, the ISRG was finishing the development of its automated CA infrastructure: the first of its kind to become publicly-trusted, under the name Let's Encrypt, which used a young protocol called ACME to automate domain validation and certificate issuance.

Meanwhile, a project called [Caddy](https://caddyserver.com) was being developed which would be the first and only web server to use HTTPS _automatically and by default_. To make that possible, another project called lego was commissioned by the Caddy project to become of the first-ever ACME client libraries, and the first client written in Go. It was made by Sebastian Erhart (xenolf), and on day 1 of Let's Encrypt's public beta, Caddy used lego to obtain its first certificate automatically at startup, making Caddy and lego the first-ever integrated ACME client.

Since then, Caddy has seen use in production longer than any other ACME client integration, and is well-known for being one of the most robust and reliable HTTPS implementations available today.

A few years later, Caddy's novel auto-HTTPS logic was extracted into a library called [CertMagic](https://github.com/caddyserver/certmagic) to be usable by any Go program. Caddy would continue to use CertMagic, which implemented the certificate _automation and management_ logic on top of the low-level certificate _obtain_ logic that lego provided.

Soon thereafter, the lego project shifted maintainership and the goals and vision of the project diverged from those of Caddy's use case of managing tens of thousands of certificates per instance. Eventually, [the original Caddy author announced work on a new ACME client library in Go](https://github.com/caddyserver/certmagic/issues/71) that exceeded Caddy's harsh requirements for large-scale enterprise deployments, lean builds, and simple API. This work finally came to fruition in 2020 as ACMEz.

---

(c) 2020 Matthew Holt
