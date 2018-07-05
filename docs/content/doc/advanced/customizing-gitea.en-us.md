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

Customizing Gitea is typically done using the `custom` folder. This is the central
place to override configuration settings, templates, etc.

If Gitea is deployed from binary, all default paths will be relative to the gitea
binary. If installed from a distribution, these paths will likely be modified to
the Linux Filesystem Standard. Gitea will create required folders, including `custom/`.
Application settings are configured in `custom/conf/app.ini`. Distributions may
provide a symlink for `custom` using `/etc/gitea/`.

- [Quick Cheat Sheet](https://docs.gitea.io/en-us/config-cheat-sheet/)
- [Complete List](https://github.com/go-gitea/gitea/blob/master/custom/conf/app.ini.sample)

If the `custom` folder can't be found next to the binary, check the `GITEA_CUSTOM`
environment variable; this can be used to override the default path to something else.
`GITEA_CUSTOM` might, for example, be set by an init script.

- [List of Environment Variables](https://docs.gitea.io/en-us/specific-variables/)

**Note:** Gitea must perform a full restart to see configuration changes.

## Customizing /robots.txt

To make Gitea serve a custom `/robots.txt` (default: empty 404), create a file called
`robots.txt` in the `custom` folder with [expected contents](http://www.robotstxt.org/).

## Serving custom public files

To make Gitea serve custom public files (like pages and images), use the folder
`custom/public/` as the webroot. Symbolic links will be followed.

For example, a file `image.png` stored in `custom/public/`, can be accessed with
the url `http://gitea.domain.tld/image.png`.

## Changing the default avatar

Place the png image at the following path: `custom/public/img/avatar\_default.png`

## Customizing Gitea pages

The `custom/templates` folder allows changing every single page of Gitea. Templates
to override can be found in the `templates` directory of Gitea source. Override by
making a copy of the file under `custom/templates` using a full path structure
matching source.

Any statement contained inside `{{` and `}}` are Gitea's templete syntax and
shouldn't be touched without fully understanding these components.

### Adding links and tabs

If all you want is to add extra links to the top navigation bar, or extra tabs to the repository view, you can put them in `extra_links.tmpl` and `extra_tabs.tmpl` inside your `custom/templates/custom/` directory.

For instance, let's say you are in Germany and must add the famously legally-required "Impressum"/about page, listing who is responsible for the site's content:
just place it under your "custom/public/" directory (for instance `custom/public/impressum.html`) and put a link to it in `custom/templates/custom/extra_links.tmpl`.

To match the current style, the link should have the class name "item", and you can use `{{AppSubUrl}}` to get the base URL:
`<a class="item" href="{{AppSubUrl}}/impressum.html">Impressum</a>`

You can add new tabs in the same way, putting them in `extra_tabs.tmpl`.
The exact HTML needed to match the style of other tabs is in the file
`templates/repo/header.tmpl`
([source in GitHub](https://github.com/go-gitea/gitea/blob/master/templates/repo/header.tmpl))

### Other additions to the page

Apart from `extra_links.tmpl` and `extra_tabs.tmpl`, there are other useful templates you can put in your `custom/templates/custom/` directory:

- `header.tmpl`, just before the end of the `<head>` tag where you can add custom CSS files for instance.
- `body_outer_pre.tmpl`, right after the start of `<body>`.
- `body_inner_pre.tmpl`, before the top navigation bar, but already inside the main container `<div class="full height">`.
- `body_inner_post.tmpl`, before the end of the main container.
- `body_outer_post.tmpl`, before the bottom `<footer>` element.
- `footer.tmpl`, right before the end of the `<body>` tag, a good place for additional Javascript.

## Customizing gitignores, labels, licenses, locales, and readmes.

Place custom files in corresponding sub-folder under `custom/options`.

## Customizing the look of Gitea

Gitea has two built-in themes, the default theme `gitea`, and a dark theme `arc-green`. To change the look of your Gitea install change the value of `DEFAULT_THEME` in the [ui](https://docs.gitea.io/en-us/config-cheat-sheet/#ui-ui) section of `app.ini` to another one of the available options.
