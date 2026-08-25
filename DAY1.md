# Gitea Day 1 - Local Setup

## Objective

Clone the Gitea repository, understand its structure, build it from source without Docker, and run it locally.

## Environment

- OS: Ubuntu on WSL
- Git: Installed
- Go: 1.26+
- Node.js: 24+
- npm: Installed
- pnpm: 11.21.0
- Make: Installed

## Repository

Official repository:

https://github.com/go-gitea/gitea

## Repository Structure

The main directories reviewed were:

- `cmd/`
- `models/`
- `modules/`
- `routers/`
- `services/`
- `web_src/`
- `templates/`
- `public/`
- `tests/`
- `docs/`

Important project files reviewed:

- `go.mod`
- `go.sum`
- `package.json`
- `Makefile`
- `README.md`

## Build

Gitea was built locally using:

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make build
```



