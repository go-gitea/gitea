# Task 1 – Gitea Local Setup

## Overview
TO THE PROJECT 
This document records my work for Task 1, which involved setting up and running the Gitea project locally without Docker.

## What I Understood About Gitea

Gitea is a self-hosted Git service written in Go. It provides features such as:

- Git repository hosting
- Code management
- Pull requests and code review
- Issues
- Project management
- Wiki
- Package registry
- CI/CD using Gitea Actions

The project is designed to run across different operating systems and architectures supported by Go.

## Repository Structure

Some important directories and files I explored were:

- `cmd/` – command-related code
- `models/` – data models
- `routers/` – application routing
- `services/` – service/business logic
- `modules/` – reusable modules
- `web_src/` – frontend source code
- `templates/` – application templates
- `public/` – public/static assets
- `options/` – configuration and localization-related resources
- `docs/` – project documentation
- `main.go` – main Go application entry point
- `go.mod` – Go module and dependency information
- `package.json` – frontend dependency and Node.js configuration
- `Makefile` – build, development, and testing commands

## Environment Used

The project was run in Ubuntu through WSL2 on an ARM64 system.

The main tools installed/configured were:

- Git
- Go
- Node.js
- pnpm
- Make

Versions used during the setup included:

```text
Git: 2.43.0
Go: 1.27.0
Node.js: 24.19.0
pnpm: 11.24.0
Make: 4.3
 

