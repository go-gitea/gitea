# Gitea Local Setup and Project Understanding

## 1. Project Overview

Gitea is an open-source, self-hosted Git service that provides Git
repository hosting and collaboration features.

It provides functionality such as:

- Git repository management
- User authentication
- Issues
- Pull requests
- Repository management
- User and organization management
- Git LFS support

Gitea is primarily written in Go. Its frontend uses JavaScript,
TypeScript, Vite, and pnpm.

## 2. Repository Structure

During the setup, I reviewed the main project directories and files.

Important components include:

- `cmd/` - Command-line commands and application entry points.
- `models/` - Data models and database-related code.
- `modules/` - Reusable application modules.
- `routers/` - HTTP routing and request handling.
- `services/` - Application services and business logic.
- `templates/` - Server-side HTML templates.
- `web_src/` - Frontend source code.
- `assets/` - Static and frontend assets.
- `docs/` - Project documentation.
- `tests/` - Test-related code.
- `custom/` - Custom configuration and application data.
- `go.mod` - Go module definition and backend dependencies.
- `go.sum` - Checksums for Go dependencies.
- `package.json` - Node.js and frontend project configuration.
- `pnpm-lock.yaml` - Locked frontend dependency versions.
- `Makefile` - Build, dependency, development, and testing commands.
- `main.go` - Main Go application entry point.

## 3. Development Environment

I set up Gitea locally using Ubuntu through WSL instead of Docker.

The environment used during the setup was:

- Ubuntu 24.04
- Go 1.27.0
- Node.js 24.19.0
- npm 11.17.0
- pnpm 11.24.0
- GNU Make 4.3
- GCC 13.3.0
- Python 3.12.3
- uv

The required Go version was checked from `go.mod`.

The Node.js and pnpm requirements were checked from `package.json`.

The repository required:

```text
Go >= the version specified in go.mod
Node.js >= 22.18.0
pnpm >= 11.0.0
