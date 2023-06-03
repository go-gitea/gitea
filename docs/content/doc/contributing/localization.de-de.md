---
date: "2021-01-22T00:00:00+02:00"
title: "Übersetzungs Richtlinien"
slug: "localization"
weight: 70
toc: false
draft: false
menu:
  sidebar:
    parent: "contributing"
    name: "Übersetzungsrichtlinien"
    weight: 70
    identifier: "localization"
---

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
* _Der_ Branch (m.), plural: die Branches
* _Das_ Issue (n.)
* _Der_ Fork (m.)
* _Das_ Repository (n.), plural: die Repositories
* _Der_ Pull-Request (m.)
* _Der_ Token (m.), plural: die Token
* _Das_ Review (n.)
* _Der_ Key (m.)
* _Der_ Committer (m.), plural: die Committer

## Weiterführende Links

Diese beiden Links sind sehr empfehlenswert:

* http://docs.translatehouse.org/projects/localization-guide/en/latest/guide/translation_guidelines_german.html
* https://docs.qgis.org/2.18/en/docs/documentation_guidelines/do_translations.html
