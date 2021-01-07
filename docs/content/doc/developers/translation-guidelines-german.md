---
date: "2016-11-08T16:00:00+02:00"
title: "German Translation Guidelines"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "developers"
    name: "German Translation Guidelines"
    weight: 70
    identifier: "german-translation-guidelines"
---

# German Translation Guidelines

> This document contains the guidelines for the german translation of gitea. 
> It is used to provide a common set of rules to make sure the translation is consistent.
> Because this is about specific parts of the german language, it is written in german.

## Allgemeines

Anrede: Wenig förmlich:
* "Du"-Form
* Keine "Amtsdeusch"-Umschreibungen, einfach so als ob man den Nutzer direkt persönlich ansprechen würde

Genauer definiert:
* "falsch" anstatt "nicht korrekt/inkorrekt"
* Benutzerkonto oder Konto? Oder Account?
* "Wende dich an ..." anstatt "kontaktiere ..."
* In der selben Zeit übersetzen (sonst wird aus "is" "war")
* Richtige Anführungszeichen verwenden. Also `"` statt `''` oder `'` oder \` oder `´`
  * `„` für beginnende Anführungszeichen, `“` für schließende Anführungszeichen

Es gelten Artikel und Worttrennungen aus dem Duden.

## Formulierungen in Modals und Buttons

Es sollten die gleichen Formulierungen auf Buttons und Modals verwendet werden.

Beispiel: Wenn der Button mit `löschen` beschriftet ist, sollte im Modal die Frage lauten `Willst du das wirklich löschen?` und nicht `Willst du das wirklich entfernen?`. Gleiches gilt für Success/Errormeldungen nach der Aktion.

## Trennungen

* Pull-Request
* Time-Tracker
* E-Mail-Adresse (siehe Duden)

## Artikeldefinitionen für Anglizismen

* _Der_ Commit (m.)
* _Der_ Branch (m.)
* _Der_ Issue (m.)
* _Der_ Fork (m.)
* _Das_ Repository (n.)
* _Der_ Pull-Request (m.)
* _Das_ Token (n.)
* _Das_ Review (n.)

## Weiterführende Links

Diese beiden Links sind sehr empfehlenswert:

* http://docs.translatehouse.org/projects/localization-guide/en/latest/guide/translation_guidelines_german.html
* https://docs.qgis.org/2.18/en/docs/documentation_guidelines/do_translations.html
