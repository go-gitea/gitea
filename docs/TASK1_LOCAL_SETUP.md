# Task 1: Gitea Local Setup & Understanding

## 1. Project Overview

Gitea is a self-hosted Git service. It provides functionality such as
Git repository hosting, issues, pull requests, and project management.

## 2. Repository Structure

- `cmd/` - application commands
- `models/` - database/data models
- `modules/` - reusable modules
- `routers/` - HTTP routing
- `services/` - application/business services
- `templates/` - UI templates
- `web_src/` - frontend source
- `public/` - frontend/static assets
- `tests/` - tests
- `docs/` - documentation
- `main.go` - application entry point
- `go.mod` - Go module/dependencies
- `Makefile` - build and development commands
- `package.json` - frontend dependencies and scripts

## 3. Environment

- Operating System: Windows
- Editor: Visual Studio Code
- Git: 2.49.0
- Go: 1.27.0
- Node.js: 22.17.1
- npm: 11.4.0
- GNU Make: 4.4.1
- Git Bash: Used for the Gitea build

## 4. Local Setup

1. Cloned the Gitea repository.
2. Installed Go.
3. Configured Go in the Windows PATH.
4. Installed GNU Make.
5. Switched to Git Bash for the build.
6. Installed frontend dependencies using pnpm.
7. Verified Vite was available.
8. Built Gitea from source.

## 5. Build Command

```bash
TAGS="bindata sqlite sqlite_unlock_notify" make build

## 6. Running Gitea

Gitea was started locally without Docker using:

./gitea.exe web

The application was accessed at:

http://localhost:3000
