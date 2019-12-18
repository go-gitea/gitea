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

# Prerequisites

The following changes are made in C:\gitea\custom\conf\app.ini:

```
RUN_USER = COMPUTERNAME$
```

Sets Gitea to run as the local system user.

COMPUTERNAME is whatever the response is from `echo %COMPUTERNAME%` on the command line. If the response is `USER-PC` then `RUN_USER = USER-PC$`

## Use absolute paths

If you use sqlite3, change the `PATH` to include the full path:

```
[database]
PATH     = c:/gitea/data/gitea.db
```

# Register as a Windows Service

To register Gitea as a Windows service, open a command prompt (cmd) as an Administrator,
then run the following command:

```
sc.exe create gitea start= auto binPath= "\"C:\gitea\gitea.exe\" web --config \"C:\gitea\custom\conf\app.ini\""
```

Do not forget to replace `C:\gitea` with the correct Gitea directory.

Open "Windows Services", search for the service named "gitea", right-click it and click on
"Run". If everything is OK, Gitea will be reachable on `http://localhost:3000` (or the port
that was configured).

## Unregister as a service

To unregister Gitea as a service, open a command prompt (cmd) as an Administrator and run:

```
sc.exe delete gitea
```
