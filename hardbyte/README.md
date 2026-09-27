# HardByte UI for Gitea

The HardByte design system on Gitea: a theme, two small templates, the brand images. Everything
HardByte adds to this fork lives in this folder; the Gitea sources are untouched.

Written against **Gitea 1.27.2**, the version production (git.hardbyte.cloud) runs. The fork's
`main` (1.28 development, 2026-08-31) has the same theme variables, so the theme works there too.

## What is here

```
hardbyte/
  custom/                          mirrors $GITEA_CUSTOM (/data/gitea in the official image)
    public/assets/css/
      theme-hardbyte-dark.css      the theme: all 295 Gitea theme variables, then a few rules
                                   where no variable reaches (links, state labels, sign-in page)
      hardbyte.css                 loaded under every theme: the layout of the start page
    public/assets/img/             the mark as logo.svg and favicon.svg; favicon.png and
                                   apple-touch-icon.png (180 px), logo.png (512 px, Open Graph),
                                   avatar_default.png, lockup-dark.png
    public/assets/fonts/           Inter 400, 500, 600, 700
    templates/home.tmpl            the signed-out start page
    templates/custom/header.tmpl   links hardbyte.css on every page
  assets/                          SVG sources (avatar-default.svg, logo-mark.svg)
  preview/compose.yml              a local Gitea to look at it
```

The values are the design system's dark theme (`tokens/colors.css`, `typography.css`,
`effects.css`), restated literally, because Gitea does not load the design system's stylesheet. A
token changed there has to be changed here.

## Decisions

Josie, 2026-09-27, after the first preview:

- **States:** open is green, merged is blue (the info tone), closed is grey. Red is left for
  danger and failure only.
- **Diffs:** green and red, as everyone reads them, but muted: the soft status grounds.
- **Signed out:** the start page and the sign-in page are the product's brand surface, with the
  lockup and the spectral line. On the sign-in page the identity provider's button (Gitea's own
  label) comes first and is the one filled button; the password form below is the break-glass
  admin's.
- **Name:** "HardByte Git".
- **Themes:** HardByte is the default; Gitea's own themes stay selectable. That is why the start
  page's layout is in `hardbyte.css` and uses Gitea's variables: it has to hold under every
  theme. On a light theme the mark stands in for the lockup, whose name is white.

## Using it with the official image

No build: mount the folder and set three values.

```yaml
volumes:
  - <this folder>/custom/public:/data/gitea/public:ro
  - <this folder>/custom/templates:/data/gitea/templates:ro
environment:
  GITEA__ui__DEFAULT_THEME: hardbyte-dark
  GITEA____APP_NAME: HardByte Git    # four underscores: the ini file's root section
```

- **Restart after changes.** Gitea reads the theme list once. Templates also load at start;
  stylesheets and images are read on each request, but browsers keep them for
  `STATIC_CACHE_TIME` (6 h by default).
- **Existing users keep their theme.** Each user's theme is stored with them, so
  `DEFAULT_THEME` only reaches new users and signed-out visitors. Existing users switch under
  Settings → Appearance, or an admin updates the stored theme once. That is to be decided at
  rollout.
- **Open:** how this folder reaches the production host (git.hardbyte.cloud is run by the
  Infrastucture repo's `gitea/compose.yml`). A custom image from this fork is also possible, but
  heavier: rebuilt for every Gitea release.
- **Optional, not decided:** a quieter footer, with `[other] SHOW_FOOTER_POWERED_BY`,
  `SHOW_FOOTER_VERSION` and `SHOW_FOOTER_TEMPLATE_LOAD_TIME` set to `false`.

## After every Gitea upgrade

1. Compare `web_src/css/themes/theme-gitea-dark.css` between the old and the new tag. A new
   variable has to be added here: Gitea's variables have no fallbacks.
2. Compare the templates this folder overrides or depends on: `templates/home.tmpl`,
   `templates/base/head.tmpl` (where `custom/header` is included), and
   `templates/user/auth/signin_inner.tmpl` together with `external_auth_methods.tmpl`.
   The sign-in rules rely on `.page-content.user.signin` and `#external-login-navigator`.
3. Look at the screens: the start page, the sign-in page, a pull request and its diff, the pull
   request list, a code view, the dashboard.

## Known limits

- **Not reachable by a theme:** user label colours (inline styles), the language bar, project
  column colours, mermaid diagrams, the API page's Swagger UI.
- **Actions run pages** show Gitea's favicon: it is compiled into the JavaScript bundle.
- **Code views** keep a monospace font. The design system has none, but code needs one.
- **The navbar logo** stays 30 px and the navbar 49 px high: both are fixed in Gitea's markup
  and CSS.
