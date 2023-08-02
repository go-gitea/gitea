---
date: "2023-01-07T22:03:00+01:00"
title: "Dokumentation"
slug: /
sidebar_position: 10
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

* Ein Raspberry Pi 3 ist leistungsstark genug, um Gitea für kleine Belastungen laufen zu lassen.
* 2 CPU Kerne und 1GB RAM sind für kleine Teams/Projekte ausreichend.
* Gitea sollte unter einem seperaten nicht-root Account auf UNIX-Systemen ausgeführt werden.
  * Achtung: Gitea verwaltet die `~/.ssh/authorized_keys` Datei. Gitea unter einem normalen Benutzer auszuführen könnte dazu führen, dass dieser sich nicht mehr anmelden kann.
* [Git](https://git-scm.com/) Version 2.0 oder aktueller wird benötigt.
  * Wenn Git >= 2.1.2 und [Git LFS](https://git-lfs.github.com/) vorhanden ist, dann wird Git LFS Support automatisch für Gitea aktiviert.
  * Wenn Git >= 2.18, dann wird das Rendern von Commit-Graphen automatisch aktiviert.

## Browser Unterstützung

* Die neuesten zwei Versionen von Chrome, Firefox, Safari und Edge
* Firefox ESR
