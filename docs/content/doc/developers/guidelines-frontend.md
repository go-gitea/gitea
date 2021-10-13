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

The HTML pages are rendered by [Go Text Template](https://pkg.go.dev/text/template)

## General Guidelines

We recommend [Google HTML/CSS Style Guide](https://google.github.io/styleguide/htmlcssguide.html) and [Google JavaScript Style Guide](https://google.github.io/styleguide/jsguide.html)

Guidelines specialized for Gitea:

1. Every feature (Fomantic-UI/jQuery module) should be put in separated files/directories.
2. HTML id/css-class-name should use kebab-case.
3. HTML id/css-class-name used by JavaScript top-level selector should be unique in whole project,
   and should contain 2-3 feature related keywords. Recommend to use `js-` prefix for CSS names for JavaScript usage only.
4. jQuery events across different features should use their own namespaces.
5. CSS styles provided by frameworks should not be overwritten by framework's selectors globally.
   Always use new defined CSS names to overwrite framework styles. Recommend to use `us-` prefix for user defined styles.  
6. Backend can pass data to frontend (JavaScript) by `ctx.PageData["myModuleData"] = map{}`
7. Simple pages and SEO-related pages use Go Text Template render to generate static Fomantic-UI HTML output. Complex pages can use Vue2 (or Vue3 in future).

## Legacy Problems and Solutions

### Too many codes in `web_src/index.js`

In history, many JavaScript codes are written into `web_src/index.js` directly, which becomes too big to maintain.
We should split this file into small modules, the separated files can be put in `web_src/js/features` for the first step.

### Vue2/Vue3 and JSX

Gitea is using Vue2 now, we have plan to upgrade to Vue3. We decide not to introduce JSX now to make sure the HTML and JavaScript codes are not mixed together.
