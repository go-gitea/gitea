---
date: "2016-11-08T16:00:00+02:00"
title: "Dokumentation"
slug: "documentation"
url: "/de-de/"
weight: 10
toc: false
draft: false
---

# Was ist Gitea?

Gitea ist ein einfacher, selbst gehosteter Git-Service. Änlich wie GitHub, Bitbucket oder GitLab.  
Gitea ist ein [Gogs](http://gogs.io)-Fork.

## Ziele

 * Einfach zu installieren
 * Plattformübergreifend
 * Leichtgewichtig
 * Quelloffen

## System Voraussetzungen

- A Raspberry Pi 3 is powerful enough to run Gitea for small workloads.
- 2 CPU cores and 1GB RAM is typically sufficient for small teams/projects.
- Gitea should be run with a dedicated non-root system account on UNIX-type systems.
   - Note: Gitea manages the `~/.ssh/authorized_keys` file. Running Gitea as a regular user could break that user's ability to log in.
- [Git](https://git-scm.com/) version 1.7.2 or later is required. Version 1.9.0 or later is recommended. Also please note:
   - Git [large file storage](https://git-lfs.github.com/) will be available if enabled when git >= 2.1.2.
   - Git commit-graph rendering will be enabled automatically when git >= 2.18.

## Browser Unterstützung

- Letzten 2 Versions von Chrome, Firefox, Safari und Edge
- Firefox ESR
