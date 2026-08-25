# Gitea Local Setup - Task 1

## Environment

- OS: Windows
- Go: 1.27.0
- Node.js: 24.18.0
- pnpm: 11.22.0
- GNU Make: 4.4.1

## What I did

1. Cloned the Gitea repository.
2. Reviewed the repository structure and documentation.
3. Configured the local development environment.
4. Installed frontend dependencies using pnpm.
5. Built Gitea from source without Docker.
6. Generated the `gitea.exe` executable.
7. Configured Gitea with SQLite3 for local use.
8. Started Gitea locally on port 3000.
9. Verified the application through `http://localhost:3000`.

## Troubleshooting

I encountered PATH and command-availability issues while setting up the development environment. Go, Node.js, Git, Make, and pnpm were not initially available in the same shell environment. I identified the issue and configured the environment so the required tools could be used.

## What I learned

I learned how to approach a large open-source project, understand its repository structure, install dependencies, build an application from source, troubleshoot environment issues, and run a web application locally without Docker.
