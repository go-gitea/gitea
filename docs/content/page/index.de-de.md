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

- Ein Raspberry Pi 3 ist leistungsstark genug, um Gitea für kleine Belastungen laufen zu lassen.
- 2 CPU Kerne und 1GB RAM sind für kleine Teams/Projekte ausreichend.
- Gitea sollte unter einem seperaten nicht-root Account auf UNIX-Systemen ausgeführt werden.
   - Achtung: Gitea verwaltet die `~/.ssh/authorized_keys` Datei. Gitea unter einem normalen Benutzer auszuführen könnte dazu führen, dass dieser sich nicht mehr anmelden kann.
- [Git](https://git-scm.com/) Version 1.7.2 oder später wird benötigt. Version 1.9.0 oder später wird empfohlen. Außerdem zu beachten:
   - Wenn git >= 2.1.2. und [Git large file storage](https://git-lfs.github.com/) aktiviert ist, dann wird es auch in Gitea verwendbar sein.
   - Wenn git >= 2.18, dann wird das Rendern von Commit-Graphen automatisch aktiviert.

## Browser Unterstützung

- Letzten 2 Versions von Chrome, Firefox, Safari und Edge
- Firefox ESR
