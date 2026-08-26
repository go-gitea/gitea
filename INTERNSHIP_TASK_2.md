\# DevOps Internship - Task 2: Automate Local Gitea Setup



\## Objective



Create a shell script that automates the process of building and running Gitea locally without manually executing each command.



\## Script



The automation script is:



`setup-gitea.sh`



\## Requirements Implemented



The script:



\- Checks whether required development tools are installed.

\- Displays installed dependency versions.

\- Verifies that it is running from the Gitea project directory.

\- Reads the required Go version from `go.mod`.

\- Builds Gitea from source using `make build`.

\- Verifies that the Gitea binary was created.

\- Checks whether port 3000 is already in use.

\- Starts the Gitea web server.

\- Displays the local Gitea URL.

\- Provides clear status and progress messages.

\- Handles errors and stops when a required step fails.

\- Does not use hard-coded user-specific paths.

\- Does not use Docker.



\## Tools Checked



The script checks:



```text

git

go

node

npm

make

pnpm

uv