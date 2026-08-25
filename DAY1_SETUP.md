# Gitea Local Setup - Day 1

## Environment
- OS: macOS (Apple Silicon M4)
- Git: Installed
- Go: Installed
- Node.js: Installed
- pnpm: Installed
- Make: Installed

## Steps Completed
1. Forked the Gitea repository.
2. Cloned the repository locally.
3. Read the project documentation.
4. Installed the required dependencies.
5. Installed pnpm to resolve the build dependency.
6. Built the project using `make build`.
7. Ran Gitea locally without Docker.
8. Verified the application at http://localhost:3000.

## Issue Faced
- `pnpm: No such file or directory`

### Resolution
Installed pnpm globally:

```bash
npm install -g pnpm


Learning
- Understood the overall Gitea project structure.
- Learned how a large Go application is organized.
- Learned how frontend assets are built using pnpm.
- Successfully ran Gitea locally without Docker.