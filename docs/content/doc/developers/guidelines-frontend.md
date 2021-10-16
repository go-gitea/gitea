---
date: "2021-10-13T16:00:00+02:00"
title: "Guidelines for Frontend Development"
slug: "guidelines-frontend"
weight: 20
toc: false
draft: false
menu:
  sidebar:
    parent: "developers"
    name: "Guidelines for Frontend"
    weight: 20
    identifier: "guidelines-frontend"
---

# Guidelines for Frontend Development

**Table of Contents**

{{< toc >}}

## Background

Gitea uses [Less CSS](https://lesscss.org), [Fomantic-UI](https://fomantic-ui.com/introduction/getting-started.html) (based on [jQuery](https://api.jquery.com)) and [Vue2](https://vuejs.org/v2/guide/) for its frontend.

The HTML pages are rendered by [Go HTML Template](https://pkg.go.dev/html/template)

## General Guidelines

We recommend [Google HTML/CSS Style Guide](https://google.github.io/styleguide/htmlcssguide.html) and [Google JavaScript Style Guide](https://google.github.io/styleguide/jsguide.html)

### Gitea specific guidelines:

1. Every feature (Fomantic-UI/jQuery module) should be put in separate files/directories.
2. HTML ids and classes should use kebab-case.
3. HTML ids and classes used in JavaScript should be unique for the whole project, and should contain 2-3 feature related keywords. We recommend to use the `js-` prefix for classes that are only used in JavaScript.
4. jQuery events across different features should use their own namespaces.
5. CSS styling for classes provided by frameworks should not be overwritten. Always use new class-names to overwrite framework styles. We recommend to use the `us-` prefix for user defined styles.  
6. The backend can pass complex data to the frontend by using `ctx.PageData["myModuleData"] = map[]{}`
7. Simple pages and SEO-related pages use Go HTML Template render to generate static Fomantic-UI HTML output. Complex pages can use Vue2 (or Vue3 in future).

### Vue2/Vue3 and JSX

Gitea is using Vue2 now, we plan to upgrade to Vue3. We decided not to introduce JSX to keep the HTML and the JavaScript code separated.
