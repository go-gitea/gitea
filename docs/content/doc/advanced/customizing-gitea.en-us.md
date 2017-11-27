---
date: "2017-04-15T14:56:00+02:00"
title: "Customizing Gitea"
slug: "customizing-gitea"
weight: 9
toc: false
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Customizing Gitea"
    weight: 9
    identifier: "customizing-gitea"
---

# Customizing Gitea

The main way to customize Gitea is by using the `custom` folder. This is the central place to override and configure features.

If you install Gitea from binary, after the installation process ends, you can find the `custom` folder next to the binary.
Gitea will create the folder for you and prepopulate it with a `conf` folder inside, where Gitea stores all the configuration settings provided through the installation steps (have a look [here](https://docs.gitea.io/en-us/config-cheat-sheet/) for a complete list).

If you can't find the `custom` folder next to the binary, please check the `GITEA_CUSTOM` environment variable, that can be used to override the default path to something else. `GITEA_CUSTOM` might be set for example in your launch script file. Please have a look [here](https://docs.gitea.io/en-us/specific-variables/) for a complete list of environment variables.

**Note** that you have to restart Gitea for it to notice the changes.

## Customizing /robots.txt

To make Gitea serve your own `/robots.txt` (by default, an empty 404 status is served), simply create a file called `robots.txt` in the `custom` folder with the [expected contents](http://www.robotstxt.org/).

## Serving custom public files

To make Gitea serve custom public files (like pages and images), use the folder `custom/public/` as the webroot. Symbolic links will be followed.

For example, a file `image.png` stored in `custom/public`, can be accessed with the url `http://your-gitea-url/image.png`.

## Changing the default avatar

Place the png image at the following path: `custom/public/img/avatar_default.png`

## Customizing Gitea pages

The `custom/templates` folder allows you to change every single page of Gitea.

You need to be aware of the template you want to change. All templates can be found in the `templates` folder of the Gitea sources.

When you find the correct .tmpl file, you need to copy it in the `custom/templates` folder of your installation, __respecting__ any subfolder you found in the source template.

You can now customize the template you copied in `custom/templates`, being carefully to not break the Gitea syntax.
Any statement contained inside `{{` and `}}` are Gitea templete's syntax and shouldn't be touch, unless you know what are you doing.

## Customizing gitignores, labels, licenses, locales, and readmes.

Place your own files in corresponding sub-folder under `custom/options`.