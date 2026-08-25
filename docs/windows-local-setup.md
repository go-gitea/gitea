# Running Gitea Locally on Windows

This guide documents the steps followed to run Gitea locally on Windows without Docker.

## Prerequisites

The local environment used for this setup included:

- Git
- Go
- Node.js
- npm
- Make
- A cloned Gitea repository

The repository documentation recommends using the Go version specified in `go.mod` and Node.js for frontend assets.

## Repository Setup

The Gitea repository was forked on GitHub and cloned locally.

The repository was then configured with:

- `origin` pointing to the personal GitHub fork
- `upstream` pointing to the official Gitea repository

## Dependencies

The Go and frontend dependencies were installed before attempting to run the application.

The frontend assets were successfully built during the setup process.

## Running Gitea Without Docker

The initial attempt was to use the Make-based build workflow. On Windows, a Make executable conflict caused the build process to fail.

The project was then built directly using Go:

    go build -o gitea.exe .

After the executable was generated, Gitea was started with:

    ./gitea.exe web

The application was then accessed locally at:

    http://localhost:3000

The Gitea web interface loaded successfully, confirming that the application was running locally without Docker.

## Verification

An initial server check using:

    curl -I http://localhost:3000

The application was successfully verified through the browser after starting the Gitea web server.

## Notes

The repository contains the Go backend, frontend source, routers, services, models, modules, templates, tests, documentation, and build tooling.

The main application entry point is `main.go`, while `go.mod` manages the Go module and dependencies.
