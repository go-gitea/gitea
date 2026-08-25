\# Gitea Local Setup



\## 1. Project Overview



Gitea is a self-hosted Git service that provides Git repository hosting and features such as repository management, issues, pull requests, user management, code collaboration and access control.



The Gitea project is primarily written in Go. Also contains frontend code using JavaScript and TypeScript.



For this task the official Gitea repository was cloned from GitHub. Configured to run locally on a Windows system without using Docker.



\---



\## 2. Repository



Official Gitea repository:



https://github.com/go-gitea/gitea



The repository was cloned to the system and the project structure was reviewed to understand the major components of the application.



\---



\## 3. Repository Structure



The following are some of the folders and files identified during the project review.



| Folder/File Description |



|---|---|



| `cmd/` | Contains application command and entry-point related code. |



| `Routers/` | Handles web and API routing.



| `Services/` | Contains business and service-layer logic.



| `Models/` | Contains database models and data-related structures.



| `Modules/` | Contains reusable internal modules. |



Web\_src/` | Contains frontend source code. |



Templates/` | Contains server-side UI templates. |



| `Public/` | Contains files and frontend assets.



| `Tests/` | Contains automated tests.



| `Docs/` | Contains project documentation.



| `Options/` | Contains application configuration options.



| `Custom/` | Contains custom application configuration and related data. |



Main.go` | Main application entry point.



| `Go.mod` | Defines the Go module and project dependencies.



| `Go.sum` | Contains checksums, for Go dependencies. |



Package.json` | Contains frontend/Node.js dependencies and scripts.



| `Makefile` | Contains build and development commands.



| `README.md` | Main project documentation. |



\---



\## 4. Environment Setup



The Gitea project was configured on a Windows development environment.



The required development tools and dependencies were set up including:



\- Go



\- Node.js



\- pnpm



\- Make



\- Python/uv where required by the project



The project dependencies were installed before building the application.



The environment was configured specifically to run Gitea without using Docker.



\---



\## 5. Build Process



After completing the environment setup the Gitea project was built using:



cmd = make build

\## 6. Local Server Setup



After the build completed successfully, Gitea was started directly on the local Windows system without using Docker.



The Gitea server was started using:



```bash

./gitea web



