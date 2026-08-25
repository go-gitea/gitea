# Gitea Local Setup

## Environment

- OS: Ubuntu 26.04 LTS on WSL2
- Go: 1.27.0
- Node.js: 24.19.0
- pnpm: 11.22.0
- SQLite: 3.46.1

## Setup Steps

1. Cloned the Gitea repository from GitHub.
2. Reviewed the project documentation and repository structure.
3. Installed the required development dependencies.
4. Installed the frontend dependencies using pnpm.
5. Built Gitea using `make build`.
6. Started Gitea locally using `./gitea web`.
7. Configured Gitea with SQLite for local development.
8. Verified the application through `http://localhost:3000`.

## Key Directories

- `cmd/` - command entry points
- `models/` - database models
- `modules/` - reusable application modules
- `routers/` - HTTP routing
- `services/` - service/business logic
- `templates/` - HTML templates
- `web_src/` - frontend source
- `docs/` - documentation
- `tests/` - testing code

## Result

Gitea was successfully built and run locally without Docker.
