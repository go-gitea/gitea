# DevOps Internship Task 1 – Gitea Local Setup

## Intern

Mujtaba Shaikh

## Date

25 August 2026

## Task

Gitea Local Setup

## Objective

The objective of this task was to clone the Gitea source repository, understand the project structure, build Gitea from source, configure it for local development, and run it successfully on Ubuntu without Docker.

## Environment

- Operating System: Ubuntu Linux
- Git
- Go
- Node.js
- pnpm
- Make
- Gitea
- SQLite

## Work Completed

- Cloned the Gitea repository from GitHub.
- Reviewed the repository structure and project documentation.
- Checked the required development tools and dependencies.
- Built Gitea from source.
- Started Gitea locally using the `./gitea web` command.
- Accessed the Gitea installation page through the browser.
- Configured Gitea for local development.
- Selected SQLite as the local database.
- Completed the initial Gitea configuration.
- Verified that Gitea was accessible through the browser.
- Successfully opened the Gitea dashboard at `http://localhost:3000`.

## Build and Run

Gitea was built locally from the source repository.

The application was started using:

```bash
./gitea web
```

The Gitea server started successfully and listened on:

```text
0.0.0.0:3000
```

The configured application URL was:

```text
http://localhost:3000/
```

## Configuration

During the first run, Gitea displayed the initial installation page because the local instance had not yet been configured.

The Gitea installation page was used to configure the local instance and select SQLite as the database.

The Gitea configuration file was generated under:

```text
custom/conf/app.ini
```

After completing the initial configuration, Gitea successfully loaded the dashboard.

## Issues Encountered

During the initial setup, Gitea displayed the installation page because the local instance had not yet been configured.

When the unconfigured Gitea instance was stopped and started again, the terminal displayed a message indicating that the configuration for an installed instance could not be loaded because the installation had not yet been completed.

The issue was resolved by completing the initial Gitea configuration through the web installation page.

After completing the configuration, Gitea started successfully and the dashboard was accessible through the browser.

## Verification

The Gitea web application was opened successfully at:

```text
http://localhost:3000
```

The Gitea dashboard was displayed correctly, confirming that the local installation was working.

## Git Verification

The repository was managed using Git throughout the task.

A separate branch named `task-1` was created for the internship work.

The task documentation was committed to the branch with the commit:

```text
docs: add DevOps internship task 1 report
```

The `task-1` branch was pushed to the personal GitHub fork, and a pull request was created from the fork's `task-1` branch to the `main` branch of the original Gitea repository.

## Key Learnings

- Learned how to work with a large open-source Git repository.
- Learned how to build Gitea from source.
- Learned how to run a Go application locally.
- Learned how to configure Gitea for local development.
- Learned how SQLite can be used for local development.
- Learned about the Gitea `app.ini` configuration file.
- Learned how to troubleshoot the initial Gitea installation configuration.
- Learned how to verify a locally running web application.
- Learned how to work with Git branches, commits, forks, and pull requests.

## Result

Gitea was successfully built, configured, and run on Ubuntu without Docker.

The application was successfully verified through:

```text
http://localhost:3000
```

The Gitea dashboard was accessible and working correctly.

The task documentation was committed to the `task-1` branch and pushed to the personal fork.

## Conclusion

Task 1 was successfully completed. Gitea was built from source, configured for local development using SQLite, and successfully run and verified through the local web interface.

The work was documented in `task-1.md`, committed to the `task-1` branch, pushed to the personal fork, and submitted through the required pull request.
