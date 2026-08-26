# Task 2 — Gitea Build and Run Automation

## Objective

Create a shell script to automate building and running the Gitea project locally without Docker.

## Requirements

- Bash
- Go
- Node.js
- pnpm
- Make
- Gitea source repository

## What the Script Does

The `build-gitea.sh` script:

1. Checks whether the required tools are installed.
2. Displays the installed dependency versions.
3. Verifies that the provided directory is a valid Gitea project.
4. Builds Gitea from source using `make build`.
5. Verifies that the Gitea binary was created successfully.
6. Checks whether port `3000` is already in use.
7. Starts the Gitea web server on port `3000`.
8. Displays the local URL.

## Steps Followed

1. Cloned the Gitea repository locally.
2. Reviewed the Gitea build documentation and project structure.
3. Verified the required development tools and versions.
4. Created the automation script.
5. Added error handling and status messages.
6. Tested the script with a valid Gitea project directory.
7. Tested error handling when the project directory was missing.
8. Tested the port check when port `3000` was already in use.
9. Successfully built and started Gitea locally using the script.

## How to Run

From the `task-2` directory:

```bash
chmod +x build-gitea.sh
./build-gitea.sh /path/to/gitea

## Verify 
```bash
http://localhost:3000
```
