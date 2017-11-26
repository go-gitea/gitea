---
date: "2016-12-21T15:00:00-02:00"
title: "Register as a Windows Service"
slug: "windows-service"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "Windows Service"
    weight: 30
    identifier: "windows-service"
---

# Register as a Windows Service

To register Gitea as a Windows service, first run `cmd` as an Administrator, and then run the following command:

```
sc create gitea start= auto binPath= ""C:\gitea\gitea.exe" web --config "C:\gitea\custom\conf\app.ini""
```

Do not forget to replace `C:\gitea` with your real Gitea folder.

After, open "Windows Services", search for the service named "gitea", right-click it and click on "Run". If everything is OK you should be able to reach Gitea on `http://localhost:3000` (or the port is was configured, if different than 3000).

## Unregister as a service

To unregister Gitea as a service, open `cmd` as an Administrator and run:

```
sc delete gitea
```
