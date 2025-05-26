
# 🐳 Run Gitea using Docker

This guide explains how to run **Gitea**, a self-hosted Git service, using **Docker**.

---

## ✅ 1. Install Docker

Download and install Docker from the official website:
👉 [https://www.docker.com/products/docker-desktop](https://www.docker.com/products/docker-desktop)

---

## ✅ 2. Create Required Folders

Create directories for persistent Gitea data:

### On Linux / macOS:
```bash
mkdir -p ~/gitea/{data,config}
```

### On Windows PowerShell:
```powershell
mkdir gitea\data
mkdir gitea\config
```

---

## ✅ 3. Run Gitea Docker Container

Use this command to run Gitea with **SQLite** (simplest setup):

```bash
docker run -d --name=gitea \
  -p 3000:3000 -p 222:22 \
  -v ~/gitea/data:/data \
  gitea/gitea:latest
```

- `-p 3000:3000`: maps web interface to `http://localhost:3000`
- `-p 222:22`: maps SSH (optional)
- `-v ~/gitea/data:/data`: mounts persistent data volume

> ⚠️ On Windows, use full path like `C:/Users/YourName/gitea/data` instead of `~/gitea/data`.

---

## ✅ 4. Access Gitea mm

After a few seconds, go to:
```
http://localhost:3000
```

Follow the setup wizard:
- Choose **SQLite** (or MySQL/PostgreSQL)
- Set admin user credentials
- Complete setup

---

## ✅ 5. (Optional) Docker Compose Setup

Create a `docker-compose.yml` file:

```yaml
version: "3"

services:
  gitea:
    image: gitea/gitea:latest
    container_name: gitea
    environment:
      - USER_UID=1000
      - USER_GID=1000
    restart: always
    volumes:
      - ./gitea/data:/data
    ports:
      - "3000:3000"
      - "222:22"
```

Then run:

```bash
docker-compose up -d
```

---

## ✅ Common URLs

- Web UI: `http://localhost:3000`
- SSH (if enabled): `ssh -p 222 git@localhost`

---

Let me know if you want to use a specific database (e.g. MySQL, PostgreSQL) or encounter any issues.
