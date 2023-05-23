---
date: "2017-04-15T14:56:00+02:00"
title: "Customizing Gitea"
slug: "customizing-gitea"
weight: 100
toc: false
draft: false
aliases:
  - /en-us/customizing-gitea
menu:
  sidebar:
    parent: "administration"
    name: "Customizing Gitea"
    identifier: "customizing-gitea"
    weight: 100
---

# Customizing Gitea

Customizing Gitea is typically done using the `CustomPath` folder - by default this is
the `custom` folder from the running directory, but may be different if your build has
set this differently. This is the central place to override configuration settings,
templates, etc. You can check the `CustomPath` using `gitea help`. You can also find
the path on the _Configuration_ tab in the _Site Administration_ page. You can override
the `CustomPath` by setting either the `GITEA_CUSTOM` environment variable or by
using the `--custom-path` option on the `gitea` binary. (The option will override the
environment variable.)

If Gitea is deployed from binary, all default paths will be relative to the Gitea
binary. If installed from a distribution, these paths will likely be modified to
the Linux Filesystem Standard. Gitea will attempt to create required folders, including
`custom/`. Distributions may provide a symlink for `custom` using `/etc/gitea/`.

Application settings can be found in file `CustomConf` which is by default,
`$GITEA_CUSTOM/conf/app.ini` but may be different if your build has set this differently.
Again `gitea help` will allow you review this variable and you can override it using the
`--config` option on the `gitea` binary.

- [Quick Cheat Sheet](https://docs.gitea.io/en-us/config-cheat-sheet/)
- [Complete List](https://github.com/go-gitea/gitea/blob/main/custom/conf/app.example.ini)

If the `CustomPath` folder can't be found despite checking `gitea help`, check the `GITEA_CUSTOM`
environment variable; this can be used to override the default path to something else.
`GITEA_CUSTOM` might, for example, be set by an init script. You can check whether the value
is set under the "Configuration" tab on the site administration page.

- [List of Environment Variables](https://docs.gitea.io/en-us/environment-variables/)

**Note:** Gitea must perform a full restart to see configuration changes.

**Table of Contents**

{{< toc >}}

## Serving custom public files

To make Gitea serve custom public files (like pages and images), use the folder
`$GITEA_CUSTOM/public/` as the webroot. Symbolic links will be followed.

For example, a file `image.png` stored in `$GITEA_CUSTOM/public/`, can be accessed with
the url `http://gitea.domain.tld/assets/image.png`.

## Changing the logo

To build a custom logo and/or favicon clone the Gitea source repository, replace `assets/logo.svg` and/or `assets/favicon.svg` and run
`make generate-images`. `assets/favicon.svg` is used for the favicon only. This will update below output files which you can then place in `$GITEA_CUSTOM/public/img` on your server:

- `public/img/logo.svg` - Used for site icon, app icon
- `public/img/logo.png` - Used for Open Graph
- `public/img/avatar_default.png` - Used as the default avatar image
- `public/img/apple-touch-icon.png` - Used on iOS devices for bookmarks
- `public/img/favicon.svg` - Used for favicon
- `public/img/favicon.png` - Used as fallback for browsers that don't support SVG favicons

In case the source image is not in vector format, you can attempt to convert a raster image using tools like [this](https://www.aconvert.com/image/png-to-svg/).

## Customizing Gitea pages and resources

Gitea's executable contains all the resources required to run: templates, images, style-sheets
and translations. Any of them can be overridden by placing a replacement in a matching path
inside the `custom` directory. For example, to replace the default `.gitignore` provided
for C++ repositories, we want to replace `options/gitignore/C++`. To do this, a replacement
must be placed in `$GITEA_CUSTOM/options/gitignore/C++` (see about the location of the `CustomPath`
directory at the top of this document).

Every single page of Gitea can be changed. Dynamic content is generated using [go templates](https://golang.org/pkg/html/template/),
which can be modified by placing replacements below the `$GITEA_CUSTOM/templates` directory.

To obtain any embedded file (including templates), the [`gitea embedded` tool]({{< relref "doc/administration/cmd-embedded.en-us.md" >}}) can be used. Alternatively, they can be found in the [`templates`](https://github.com/go-gitea/gitea/tree/main/templates) directory of Gitea source (Note: the example link is from the `main` branch. Make sure to use templates compatible with the release you are using).

Be aware that any statement contained inside `{{` and `}}` are Gitea's template syntax and
shouldn't be touched without fully understanding these components.

### Customizing startpage / homepage

Copy [`home.tmpl`](https://github.com/go-gitea/gitea/blob/main/templates/home.tmpl) for your version of Gitea from `templates` to `$GITEA_CUSTOM/templates`.
Edit as you wish.
Dont forget to restart your Gitea to apply the changes.

### Adding links and tabs

If all you want is to add extra links to the top navigation bar or footer, or extra tabs to the repository view, you can put them in `extra_links.tmpl` (links added to the navbar), `extra_links_footer.tmpl` (links added to the left side of footer), and `extra_tabs.tmpl` inside your `$GITEA_CUSTOM/templates/custom/` directory.

For instance, let's say you are in Germany and must add the famously legally-required "Impressum"/about page, listing who is responsible for the site's content:
just place it under your "$GITEA_CUSTOM/public/" directory (for instance `$GITEA_CUSTOM/public/impressum.html`) and put a link to it in either `$GITEA_CUSTOM/templates/custom/extra_links.tmpl` or `$GITEA_CUSTOM/templates/custom/extra_links_footer.tmpl`.

To match the current style, the link should have the class name "item", and you can use `{{AppSubUrl}}` to get the base URL:
`<a class="item" href="{{AppSubUrl}}/assets/impressum.html">Impressum</a>`

For more information, see [Adding Legal Pages](https://docs.gitea.io/en-us/adding-legal-pages).

You can add new tabs in the same way, putting them in `extra_tabs.tmpl`.
The exact HTML needed to match the style of other tabs is in the file
`templates/repo/header.tmpl`
([source in GitHub](https://github.com/go-gitea/gitea/blob/main/templates/repo/header.tmpl))

### Other additions to the page

Apart from `extra_links.tmpl` and `extra_tabs.tmpl`, there are other useful templates you can put in your `$GITEA_CUSTOM/templates/custom/` directory:

- `header.tmpl`, just before the end of the `<head>` tag where you can add custom CSS files for instance.
- `body_outer_pre.tmpl`, right after the start of `<body>`.
- `body_inner_pre.tmpl`, before the top navigation bar, but already inside the main container `<div class="full height">`.
- `body_inner_post.tmpl`, before the end of the main container.
- `body_outer_post.tmpl`, before the bottom `<footer>` element.
- `footer.tmpl`, right before the end of the `<body>` tag, a good place for additional JavaScript.

#### Example: PlantUML

You can add [PlantUML](https://plantuml.com/) support to Gitea's markdown by using a PlantUML server.
The data is encoded and sent to the PlantUML server which generates the picture. There is an online
demo server at http://www.plantuml.com/plantuml, but if you (or your users) have sensitive data you
can set up your own [PlantUML server](https://plantuml.com/server) instead. To set up PlantUML rendering,
copy JavaScript files from https://gitea.com/davidsvantesson/plantuml-code-highlight and put them in your
`$GITEA_CUSTOM/public` folder. Then add the following to `custom/footer.tmpl`:

```html
<script>
  $(async () => {
    if (!$('.language-plantuml').length) return;
    await Promise.all([
      $.getScript('https://your-gitea-server.com/assets/deflate.js'),
      $.getScript('https://your-gitea-server.com/assets/encode.js'),
      $.getScript('https://your-gitea-server.com/assets/plantuml_codeblock_parse.js'),
    ]);
    // Replace call with address to your plantuml server
    parsePlantumlCodeBlocks("https://www.plantuml.com/plantuml");
  });
</script>
```

You can then add blocks like the following to your markdown:

```plantuml
Alice -> Bob: Authentication Request
Bob --> Alice: Authentication Response

Alice -> Bob: Another authentication Request
Alice <-- Bob: Another authentication Response
```

The script will detect tags with `class="language-plantuml"`, but you can change this by providing a second argument to `parsePlantumlCodeBlocks`.

#### Example: STL Preview

You can display STL file directly in Gitea by adding:

```html
<script>
  function lS(src) {
    return new Promise(function (resolve, reject) {
      let s = document.createElement("script");
      s.src = src;
      s.addEventListener("load", () => {
        resolve();
      });
      document.body.appendChild(s);
    });
  }

  if ($('.view-raw>a[href$=".stl" i]').length) {
    $("body").append(
      '<link href="/assets/Madeleine.js/src/css/Madeleine.css" rel="stylesheet">'
    );
    Promise.all([
      lS("/assets/Madeleine.js/src/lib/stats.js"),
      lS("/assets/Madeleine.js/src/lib/detector.js"),
      lS("/assets/Madeleine.js/src/lib/three.min.js"),
      lS("/assets/Madeleine.js/src/Madeleine.js"),
    ]).then(function () {
      $(".view-raw")
        .attr("id", "view-raw")
        .attr("style", "padding: 0;margin-bottom: -10px;");
      new Madeleine({
        target: "view-raw",
        data: $('.view-raw>a[href$=".stl" i]').attr("href"),
        path: "/assets/Madeleine.js/src",
      });
      $('.view-raw>a[href$=".stl"]').remove();
    });
  }
</script>
```

to the file `templates/custom/footer.tmpl`

You also need to download the content of the library [Madeleine.js](https://github.com/beige90/Madeleine.js) and place it under `$GITEA_CUSTOM/public/` folder.

You should end-up with a folder structure similar to:

```
$GITEA_CUSTOM/templates
-- custom
    `-- footer.tmpl
$GITEA_CUSTOM/public
-- Madeleine.js
   |-- LICENSE
   |-- README.md
   |-- css
   |   |-- pygment_trac.css
   |   `-- stylesheet.css
   |-- examples
   |   |-- ajax.html
   |   |-- index.html
   |   `-- upload.html
   |-- images
   |   |-- bg_hr.png
   |   |-- blacktocat.png
   |   |-- icon_download.png
   |   `-- sprite_download.png
   |-- models
   |   |-- dino2.stl
   |   |-- ducati.stl
   |   |-- gallardo.stl
   |   |-- lamp.stl
   |   |-- octocat.stl
   |   |-- skull.stl
   |   `-- treefrog.stl
   `-- src
       |-- Madeleine.js
       |-- css
       |   `-- Madeleine.css
       |-- icons
       |   |-- logo.png
       |   |-- madeleine.eot
       |   |-- madeleine.svg
       |   |-- madeleine.ttf
       |   `-- madeleine.woff
       `-- lib
           |-- MadeleineConverter.js
           |-- MadeleineLoader.js
           |-- detector.js
           |-- stats.js
           `-- three.min.js
```

Then restart Gitea and open a STL file on your Gitea instance.

## Customizing Gitea mails

The `$GITEA_CUSTOM/templates/mail` folder allows changing the body of every mail of Gitea.
Templates to override can be found in the
[`templates/mail`](https://github.com/go-gitea/gitea/tree/main/templates/mail)
directory of Gitea source.
Override by making a copy of the file under `$GITEA_CUSTOM/templates/mail` using a
full path structure matching source.

Any statement contained inside `{{` and `}}` are Gitea's template
syntax and shouldn't be touched without fully understanding these components.

## Adding Analytics to Gitea

Google Analytics, Matomo (previously Piwik), and other analytics services can be added to Gitea. To add the tracking code, refer to the `Other additions to the page` section of this document, and add the JavaScript to the `$GITEA_CUSTOM/templates/custom/header.tmpl` file.

## Customizing gitignores, labels, licenses, locales, and readmes.

Place custom files in corresponding sub-folder under `custom/options`.

**NOTE:** The files should not have a file extension, e.g. `Labels` rather than `Labels.txt`

### gitignores

To add custom .gitignore, add a file with existing [.gitignore rules](https://git-scm.com/docs/gitignore) in it to `$GITEA_CUSTOM/options/gitignore`

## Customizing the git configuration

Starting with Gitea 1.20, you can customize the git configuration via the `git.config` section.

### Enabling signed git pushes

To enable signed git pushes, set these two options:

```ini
[git.config]
receive.advertisePushOptions = true
receive.certNonceSeed = <randomstring>
```

`certNonceSeed` should be set to a random string and be kept secret.

### Labels

Starting with Gitea 1.19, you can add a file that follows the [YAML label format](https://github.com/go-gitea/gitea/blob/main/options/label/Advanced.yaml) to `$GITEA_CUSTOM/options/label`:

```yaml
labels:
  - name: "foo/bar"  # name of the label that will appear in the dropdown
    exclusive: true # whether to use the exclusive namespace for scoped labels. scoped delimiter is /
    color: aabbcc # hex colour coding
    description: Some label # long description of label intent
 ```

The [legacy file format](https://github.com/go-gitea/gitea/blob/main/options/label/Default) can still be used following the format below, however we strongly recommend using the newer YAML format instead.

`#hex-color label name ; label description`

For more information, see the [labels documentation]({{< relref "doc/usage/labels.en-us.md" >}}).

### Licenses

To add a custom license, add a file with the license text to `$GITEA_CUSTOM/options/license`

### Locales

Locales are managed via our [Crowdin](https://crowdin.com/project/gitea).
You can override a locale by placing an altered locale file in `$GITEA_CUSTOM/options/locale`.
Gitea's default locale files can be found in the [`options/locale`](https://github.com/go-gitea/gitea/tree/main/options/locale) source folder and these should be used as examples for your changes.

To add a completely new locale, as well as placing the file in the above location, you will need to add the new lang and name to the `[i18n]` section in your `app.ini`. Keep in mind that Gitea will use those settings as **overrides**, so if you want to keep the other languages as well you will need to copy/paste the default values and add your own to them.

```
[i18n]
LANGS = en-US,foo-BAR
NAMES = English,FooBar
```

The first locale will be used as the default if user browser's language doesn't match any locale in the list.

Locales may change between versions, so keeping track of your customized locales is highly encouraged.

### Readmes

To add a custom Readme, add a markdown formatted file (without an `.md` extension) to `$GITEA_CUSTOM/options/readme`

**NOTE:** readme templates support **variable expansion**.
currently there are `{Name}` (name of repository), `{Description}`, `{CloneURL.SSH}`, `{CloneURL.HTTPS}` and `{OwnerName}`

### Reactions

To change reaction emoji's you can set allowed reactions at app.ini

```
[ui]
REACTIONS = +1, -1, laugh, confused, heart, hooray, eyes
```

A full list of supported emoji's is at [emoji list](https://gitea.com/gitea/gitea.com/issues/8)

## Customizing the look of Gitea

The default built-in themes are `gitea` (light), `arc-green` (dark), and `auto` (chooses light or dark depending on operating system settings).
The default theme can be changed via `DEFAULT_THEME` in the [ui](https://docs.gitea.io/en-us/config-cheat-sheet/#ui-ui) section of `app.ini`.

Gitea also has support for user themes, which means every user can select which theme should be used.
The list of themes a user can choose from can be configured with the `THEMES` value in the [ui](https://docs.gitea.io/en-us/config-cheat-sheet/#ui-ui) section of `app.ini`.

To make a custom theme available to all users:

1. Add a CSS file to `$GITEA_CUSTOM/public/css/theme-<theme-name>.css`.
  The value of `$GITEA_CUSTOM` of your instance can be queried by calling `gitea help` and looking up the value of "CustomPath".
2. Add `<theme-name>` to the comma-separated list of setting `THEMES` in `app.ini`

Community themes are listed in [gitea/awesome-gitea#themes](https://gitea.com/gitea/awesome-gitea#themes).

The `arc-green` theme source can be found [here](https://github.com/go-gitea/gitea/blob/main/web_src/css/themes/theme-arc-green.css).

If your custom theme is considered a dark theme, set the global css variable `--is-dark-theme` to `true`.
This allows Gitea to adjust the Monaco code editor's theme accordingly.

## Customizing fonts

Fonts can be customized using CSS variables:

```css
:root {
  --fonts-proportional: /* custom proportional fonts */ !important;
  --fonts-monospace: /* custom monospace fonts */ !important;
  --fonts-emoji: /* custom emoji fonts */ !important;
}
```
