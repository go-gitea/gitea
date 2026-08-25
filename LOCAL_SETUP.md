# Local Gitea Setup Verification

## Environment
- OS: Linux/WSL
- Go: $(go version)
- Node: $(node -v)
- Database: SQLite3

## Steps Followed
1. Installed prerequisites (Go 1.23+, Node 22, pnpm, make, gcc, sqlite3).
2. Cloned the repository and checked out a feature branch.
3. Ran `make node_modules` to install frontend dependencies.
4. Ran `make build` to compile the backend and frontend assets.
5. Started the server with `./gitea web`.
6. Completed the web installer using SQLite3.
7. Verified functionality by creating a test repository and hitting the `/api/v1/version` endpoint.

## Issues Resolved
- Fixed pnpm Node.js version mismatch by upgrading to Node 22 via nvm.
