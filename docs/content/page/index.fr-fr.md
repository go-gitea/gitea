---
date: "2017-08-23T09:00:00+02:00"
title: "Documentation"
slug: "documentation"
url: "/fr-fr/"
weight: 10
toc: true
draft: false
---

# A propos de Gitea

Gitea est un service Git auto-hébergé très simple à installer et à utiliser. Il est similaire à GitHub, Bitbucket ou Gitlab. Le développement initial provient sur [Gogs] (http://gogs.io), mais nous l'avons forké puis renommé Gitea. Si vous souhaitez en savoir plus sur les raisons pour lesquelles nous avons fait cela, lisez [cette publication] (https://blog.gitea.io/2016/12/welcome-to-gitea/) sur le blog.

## Objectif

Le but de ce projet est de fournir de la manière la plus simple, la plus rapide et sans complication un service Git auto-hébergé. Grâce à Go, cela peut se faire via un binaire indépendant fonctionnant sur toutes les plateformes que Go prend en charge, y compris Linux, macOS et Windows, même sur des architectures comme ARM ou PowerPC.

## Fonctionalités

- Tableau de bord de l'utilisateur
    - Choix du contexte (organisation ou utilisateur actuel)
    - Chronologie de l'activité
        - Révisions (_Commits_)
        - Tickets
        - Demande d'ajout (_Pull request_)
        - Création de dépôts
    - Liste des dépôts
    - Liste de vos organisations
    - Liste des dépôts miroires
- Tableau de bord des tickets
    - Choix du contexte (organisation ou utilisateur actuel)
    - Filtres
        - Ouvert
        - Fermé
        - Vos dépôts
        - Tickets assignés
        - Vos tickets
        - Dépôts
    - Options de tri
        - Plus vieux
        - Dernière mise à jour
        - Nombre de commentaires
- Tableau de bord des demandes d'ajout
    - Identique au tableau de bord des tickets
- Types de dépôt
    - Miroire
    - Normal
    - Migré
- Notifications (courriel et web)
    - Lu
    - Non lu
    - Épinglé
- Page d'exploration
    - Utilisateurs
    - Dépôts
    - Organisations
    - Moteur de recherche
- Interface personnalisables
- Fichiers publiques remplaçables (logo, css, etc)
- Protection CSRF et XSS
- Support d'HTTPS
- Configuration des types et de la taille maximale des fichiers téléversés
- Journalisation (_Log_)
- Configuration
    - Base de données
        - MySQL
        - PostgreSQL
        - SQLite3
        - MSSQL
        - [TiDB](https://github.com/pingcap/tidb) (expérimental)
    - Fichier de configuration
        - Voir [ici](https://github.com/go-gitea/gitea/blob/master/custom/conf/app.ini.sample)
    - Panel d'administration
        - Statistiques
        - Actions
            - Suppression des comptes inactifs
            - Suppression des dépôts archivés
            - Suppression des dépôts pour lesquels il manque leurs fichiers
            - Exécution du _garbage collector_ sur les dépôts
            - Ré-écriture des clefs SSH
            - Resynchronisation des hooks
            - Recreation des dépôts manquants
        - Status du server
            - Temps de disponibilité
            - Mémoire
            - Nombre de goroutines
            - et bien plus...
         - Gestion des utilisateurs
            - Recherche
            - Tri
            - Dernière connexion
            - Méthode d'authentification
            - Nombre maximum de dépôts
            - Désactivation du compte
            - Permissions d'administration
            - Permission pour crééer des hooks
            - Permission pour crééer des organisations
            - Permission pour importer des dépôts
        - Gestion des organisations
            - Membres
            - Équipes
            - Avatar
            - Hooks
        - Gestion des depôts
            - Voir toutes les informations pour un dépôt donné et gérer tous les dépôts
        - Méthodes d'authentification
            - OAuth
            - PAM
            - LDAP
            - SMTP
        - Visualisation de la configuration
            - Tout ce que contient le fichier de configuration
        - Alertes du système
            - Quand quelque chose d'inattendu survient
        - Surveillance
            - Processus courrants
            - Tâches CRON
                - Mise à jour des dépôts miroires
                - Vérification de l'état des dépôts
                - Vérification des statistiques des dépôts
                - Nettoyage des anciennes archives
    - Variables d'environement
    - Options de ligne de commande
- Internationalisation ([21 langues](https://github.com/go-gitea/gitea/tree/master/options/locale))
- Courriel
    - Notifications
    - Confirmation d'inscription
    - Ré-initialisation du mot de passe
- Support de _reverse proxy_
    - _subpaths_ inclus
- Utilisateurs
    - Profil
        - Nom
        - Prénom
        - Courriel
        - Site internet
        - Date de création
        - Abonnés et abonnements
        - Organisations
        - Dépôts
        - Activité
        - Dépôts suivis
    - Paramètres
        - Identiques au profil avec en plus les éléments ci-dessous
        - Rendre l'adresse de courriel privée
        - Avatar
            - Gravatar
            - Libravatar
            - Personnalisé
        - Mot de passe
        - Courriels multiples
        - Clefs SSH
        - Applications connectées
        - Authentification à double facteurs
        - Identités OAuth2 attachées
        - Suppression du compte
- Dépôts
    - Clone à partir de SSH / HTTP / HTTPS
    - Git LFS
    - Suivre, Voter, Fork
    - Voir les personnes qui suivent, les votes et les forks
    - Code
        - Navigation entre les branches
        - Création ou téléversement de fichier depuis le navigateur
        - URLs pour clôner le dépôt
        - Téléchargement
            - ZIP
            - TAR.GZ
        - Édition en ligne
            - Éditeur Markdown
            - Éditeur de texte
                - Coloration syntaxique
            - Visualisation des Diffs
            - Visualisation
            - Possibilité de choisir où sauvegarder la révision
        - Historiques des fichiers
        - Suppression de fichiers
        - Voir le fichier brut
    - Tickets
        - Modèle de ticket
        - Jalons
        - Étiquettes
        - Affecter des tickets
        - Filtres
            - Ouvert
            - Ferme
            - Personne assignée
            - Créer par vous
            - Qui vous mentionne
        - Tri
            - Plus vieux
            - Dernière mise à jour
            - Nombre de commentaires
        - Moteur de recherche
        - Commentaires
        - Joindre des fichiers
    - Demande d’ajout (_Pull request_)
        - Les mêmes fonctionnalités que pour les tickets
    - Révisions (_Commits_)
        - Representation graphique des révisions
        - Révisions par branches
        - Moteur de recherche
        - Voir les différences
        - Voir les numéro de révision SHA
        - Voir l'auteur
        - Naviguer dans les fichiers d'une révision donnée
    - Publication
        - Pièces jointes
        - Titre
        - Contenu
        - Suppression
        - Définir comme une pré-publication
        - Choix de la branche
    - Wiki
        - Import
        - Éditeur Markdown
    - Paramètres
        - Options
            - Nom
            - Description
            - Privé / Publique
            - Site internet
            - Wiki
                - Activé / Désactivé
                - Interne / externe
            - Tickets
                - Activé / Désactivé
                - Interne / externe
                - URL personnalisable pour une meilleur intégration avec un gestionnaire de tickets externe
            - Activer / désactiver les demandes d'ajout (_Pull request_)
            - Transfert du dépôt
            - Suppression du wiki
            - Suppression du dépôt
        - Collaboration
            - Lecture / Écriture / Administration
        - Branches
            - Branche par défaut
            - Protection
        - Webhooks
        - Git hooks
        - Clefs de déploiement

## Configuration requise

- Un simple Raspberry Pi est assez puissant pour les fonctionnalités de base.
- Un processeur double coeurs et 1Gb de RAM est une bonne base pour une utilisation en équipe.
- Gitea est censé être exécuté avec un compte utilisateur dédié et non root, aucun autre mode de fonctionnement n'est pris en charge. (**NOTE**: Dans le cas où vous l'exécutez avec votre propre compte d'utilisateur et que le serveur SSH intégré est désactivé, Gitea modifie le fichier `~ /.ssh /authorized_keys` afin que vous ne soyez **plus capable** de vous connecter interactivement).

## Navigateurs supportés

- Consultez [Semantic UI](https://github.com/Semantic-Org/Semantic-UI#browser-support) pour la liste des navigateurs supportés.
- La taille minimale supportée officielement est de **1024*768**, l'interface utilisateur peut toujours fonctionner à une taille plus petite, mais ce n'est pas garanti et les problèmes remontés ne seront pas corrigés.

## Composants

* Framework web : [Macaron](http://go-macaron.com/)
* ORM : [XORM](https://github.com/go-xorm/xorm)
* Interface graphique :
  * [Semantic UI](http://semantic-ui.com/)
  * [GitHub Octicons](https://octicons.github.com/)
  * [Font Awesome](http://fontawesome.io/)
  * [DropzoneJS](http://www.dropzonejs.com/)
  * [Highlight](https://highlightjs.org/)
  * [Clipboard](https://zenorocha.github.io/clipboard.js/)
  * [Emojify](https://github.com/Ranks/emojify.js)
  * [CodeMirror](https://codemirror.net/)
  * [jQuery Date Time Picker](https://github.com/xdan/datetimepicker)
  * [jQuery MiniColors](https://github.com/claviska/jquery-minicolors)
* Connecteurs de base de données :
  * [github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
  * [github.com/lib/pq](https://github.com/lib/pq)
  * [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)
  * [github.com/pingcap/tidb](https://github.com/pingcap/tidb)
  * [github.com/denisenkom/go-mssqldb](https://github.com/denisenkom/go-mssqldb)

## Logiciels et services

- [Drone](https://github.com/drone/drone) (Intégration continue)
