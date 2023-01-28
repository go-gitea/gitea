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

Gitea uses [Less CSS](https://lesscss.org), [Fomantic-UI](https://fomantic-ui.com/introduction/getting-started.html) (based on [jQuery](https://api.jquery.com)) and [Vue3](https://vuejs.org/) for its frontend.

The HTML pages are rendered by [Go HTML Template](https://pkg.go.dev/html/template).

The source files can be found in the following directories:

* **Less styles:** `web_src/less/`
* **JavaScript files:** `web_src/js/`
* **Vue components:** `web_src/js/components/`
* **Go HTML templates:** `templates/`

## General Guidelines

We recommend [Google HTML/CSS Style Guide](https://google.github.io/styleguide/htmlcssguide.html) and [Google JavaScript Style Guide](https://google.github.io/styleguide/jsguide.html)

### Gitea specific guidelines:

1. Every feature (Fomantic-UI/jQuery module) should be put in separate files/directories.
2. HTML ids and classes should use kebab-case.
3. HTML ids and classes used in JavaScript should be unique for the whole project, and should contain 2-3 feature related keywords. We recommend to use the `js-` prefix for classes that are only used in JavaScript.
4. jQuery events across different features could use their own namespaces if there are potential conflicts.
5. CSS styling for classes provided by frameworks should not be overwritten. Always use new class-names with 2-3 feature related keywords to overwrite framework styles.
6. The backend can pass complex data to the frontend by using `ctx.PageData["myModuleData"] = map[]{}`
7. Simple pages and SEO-related pages use Go HTML Template render to generate static Fomantic-UI HTML output. Complex pages can use Vue3.

### Framework Usage

Mixing different frameworks together is discouraged, it makes the code difficult to be maintained.
A JavaScript module should follow one major framework and follow the framework's best practice.

Recommended implementations:

* Vue + Vanilla JS
* Fomantic-UI (jQuery)
* Vanilla JS

Discouraged implementations:

* Vue + Fomantic-UI (jQuery)
* jQuery + Vanilla JS

To make UI consistent, Vue components can use Fomantic-UI CSS classes.
Although mixing different frameworks is discouraged,
it should also work if the mixing is necessary and the code is well-designed and maintainable.

### `async` Functions

Only mark a function as `async` if and only if there are `await` calls
or `Promise` returns inside the function.

It's not recommended to use `async` event listeners, which may lead to problems.
The reason is that the code after await is executed outside the event dispatch.
Reference: https://github.com/github/eslint-plugin-github/blob/main/docs/rules/async-preventdefault.md

If we want to call an `async` function in a non-async context,
it's recommended to use `const _promise = asyncFoo()` to tell readers
that this is done by purpose, we want to call the async function and ignore the Promise.
Some lint rules and IDEs also have warnings if the returned Promise is not handled.

### HTML Attributes and `dataset`

The usage of `dataset` is forbidden, its camel-casing behaviour makes it hard to grep for attributes.
However, there are still some special cases, so the current guideline is:

* For legacy code:
  * `$.data()` should be refactored to `$.attr()`.
  * `$.data()` can be used to bind some non-string data to elements in rare cases, but it is highly discouraged.

* For new code:
  * `node.dataset` should not be used, use `node.getAttribute` instead.
  * never bind any user data to a DOM node, use a suitable design pattern to describe the relation between node and data.

### Legacy Code

A lot of legacy code already existed before this document's written. It's recommended to refactor legacy code to follow the guidelines.

### Vue3 and JSX

Gitea is using Vue3 now. We decided not to introduce JSX to keep the HTML and the JavaScript code separated.
