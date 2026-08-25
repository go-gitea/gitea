# Gitea Local Setup on Windows

## Overview

This repository documents my local setup and development environment configuration for the [Gitea](https://github.com/go-gitea/gitea) project on Windows.

The objective of this task was to fork the Gitea repository, configure the required development dependencies, build the frontend and backend components, and successfully run Gitea locally without using Docker.

---

## 1. Repository Setup

### Fork the Repository

I first forked the official Gitea repository to my personal GitHub account.

Official repository:

```text
https://github.com/go-gitea/gitea
```

After creating the fork, I created a local project directory named:

```text
myproject1
```

The repository was then cloned into this directory.

Local project structure:

```text
D:\
└── myproject1\
    └── gitea\
```

---

## 2. Create Development Branch

After cloning the repository, I created a separate development branch named:

```text
feature/main
```

This branch was used for the local development and setup work instead of making changes directly on the main branch.

---

## 3. Install Required Dependencies

I configured the required development environment on Windows.

The main dependencies required for the Gitea development setup included:

* Git
* Go
* Node.js
* npm
* Make
* Other project-specific dependencies required by Gitea

I verified the installed tools and configured the required environment variables and paths.

---

## 4. Build the Frontend

After setting up the required dependencies, I built the Gitea frontend using the project's Makefile/build process.

The frontend build was completed successfully.

The frontend build process ensures that the required JavaScript, CSS, and other frontend assets are generated for the Gitea application.

---

## 5. Configure Environment Variables

After the frontend setup, I configured the required environment variables for the local development environment.

During this process, I configured Go-related paths so that temporary files and build cache could be stored on the appropriate disk.

For example:

```text
GOTMPDIR
GOCACHE
```

These configurations helped manage disk usage during the Gitea build process.

---

## 6. Build the Backend

After completing the frontend setup and environment configuration, I built the Gitea backend using:

```bash
make backend
```

The backend build compiles the Go source code and generates the Gitea executable.

After resolving the encountered directory configuration issue, the backend was successfully built.

---

## 7. Run the Application

Once both the frontend and backend were successfully built, I ran the Gitea application locally.

The application was then accessed through the local development server.

```text
http://localhost:3000
```

This confirmed that the Gitea application was successfully running on my local Windows system without Docker.

---

# Issues Faced and Resolutions

## Issue 1: Insufficient Disk Space

### Problem

While installing dependencies/building the project, I encountered an error indicating that there was not enough available disk space.

Example:

```text
There is not enough space on the disk.
```

### Resolution

I checked the available disk space on the system and cleared unnecessary files to free up storage.

After freeing sufficient disk space, I repeated the dependency installation/build process and it completed successfully.

### Learning

Large projects such as Gitea can require significant temporary storage during dependency installation and compilation. It is therefore important to verify available disk space before starting a large build.

---

## Issue 2: Go Temporary Directory Configuration

### Problem

While running:

```bash
make backend
```

I encountered an error similar to:

```text
go: creating work dir: GetFileAttributesEx D:\Gitea\go-tmp:
The system cannot find the file specified.
```

The issue occurred because the repository was initially cloned under one directory, while the Go temporary directory environment variable was configured to point to a different directory.

The repository was located at:

```text
D:\myproject1\gitea
```

while the Go temporary directory was configured as:

```text
D:\Gitea\go-tmp
```

Since the configured directory did not exist, Go was unable to create its temporary build files.

### Resolution

I corrected the Go temporary directory configuration and pointed it to a valid directory on the same development drive.

For example:

```text
D:\myproject1\go-tmp
```

I also configured the Go build cache appropriately:

```text
D:\myproject1\go-build
```

After correcting these paths, I ran the backend build again:

```bash
make backend
```

The build completed successfully.

### Learning

Environment variables such as `GOTMPDIR` and `GOCACHE` must point to valid directories. When changing project locations, previously configured environment variables should also be checked and updated accordingly.

---

# Final Result

The Gitea project was successfully configured and built locally on Windows without Docker.

The completed workflow was:

```text
Fork Gitea Repository
        ↓
Create local myproject1 directory
        ↓
Clone Gitea repository
        ↓
Create feature/main branch
        ↓
Install dependencies
        ↓
Build Frontend
        ↓
Configure Environment Variables
        ↓
Build Backend
        ↓
Run Gitea
        ↓
Verify on localhost:3000
```

---

# Key Learnings

Through this setup, I learned:

* How to fork and clone a large open-source Git repository.
* How to create and work with Git branches.
* How to configure a Go-based development environment on Windows.
* How frontend and backend components are built separately.
* How Makefiles are used to automate project build tasks.
* How environment variables such as `GOTMPDIR` and `GOCACHE` affect Go builds.
* How to troubleshoot disk-space-related build failures.
* How incorrect directory/environment configurations can cause build failures.
* How to build and run a large open-source application locally without Docker.
* How to document technical issues and their resolutions during development.

---

# Technology Stack

The local development environment used:

* **Gitea**
* **Go**
* **Node.js**
* **npm**
* **Git**
* **Make**
* **Windows**
* **GitHub**

---

## Conclusion

The Gitea local development environment was successfully configured on Windows. The frontend and backend were built successfully, the encountered disk-space and directory configuration issues were resolved, and the application was run locally without Docker.

This task provided practical experience with open-source project setup, dependency management, environment configuration, troubleshooting, Git workflows, and building a large Go-based application.
