# Local Setup Notes

Steps I followed to build and run Gitea locally without Docker:
1. Installed Go, Node.js, pnpm, build-essential
2. Built with: TAGS="bindata sqlite sqlite_unlock_notify" make build
3. Configured custom/conf/app.ini with SQLite
4. Ran ./gitea web and registered admin account

Issues faced:
- pnpm was missing; installed via corepack
