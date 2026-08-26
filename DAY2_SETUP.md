# Gitea Local Setup Automation - Day 2

## Task

Automate the Gitea local setup process using a shell script.

## Objective

Create a shell script that builds and runs Gitea locally without Docker.

## Requirements Completed

- Check whether required tools are installed.
- Display dependency versions.
- Verify the script is running from the correct project directory.
- Build Gitea from source.
- Verify that the Gitea binary was created successfully.
- Check whether port 3000 is already in use.
- Start the Gitea web server.
- Display the local URL.
- Add error handling.
- Show clear progress messages.
- Avoid hard-coded user-specific paths.
- Do not use Docker.

## Tools Checked

The script checks for:

- Git
- Go
- Node.js
- pnpm
- Make

## Setup Process

The script performs the following steps:

1. Verifies the current Gitea project directory.
2. Checks whether required tools are installed.
3. Displays the installed dependency versions.
4. Builds Gitea from source.
5. Verifies that the Gitea binary was created.
6. Checks whether port 3000 is already in use.
7. Starts the Gitea web server.
8. Displays the local URL.

## Port Check

The script checks port 3000 before starting Gitea.

If port 3000 is already occupied, the script displays the process using the port instead of starting another server.

Example:

```text
[5/7] Checking port 3000...
ERROR: Port 3000 is already in use.