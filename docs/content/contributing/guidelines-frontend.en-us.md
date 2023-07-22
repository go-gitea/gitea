---
date: "2021-10-13T16:00:00+02:00"
title: "Guidelines for Frontend Development"
slug: "guidelines-frontend"
sidebar_position: 30
toc: false
draft: false
aliases:
  - /en-us/guidelines-frontend
menu:
  sidebar:
    parent: "contributing"
    name: "Guidelines for Frontend"
    sidebar_position: 30
    identifier: "guidelines-frontend"
---

# Guidelines for Frontend Development

## Background

Gitea uses [Fomantic-UI](https://fomantic-ui.com/introduction/getting-started.html) (based on [jQuery](https://api.jquery.com)) and [Vue3](https://vuejs.org/) for its frontend.

The HTML pages are rendered by [Go HTML Template](https://pkg.go.dev/html/template).

The source files can be found in the following directories:

* **CSS styles:** `web_src/css/`
* **JavaScript files:** `web_src/js/`
* **Vue components:** `web_src/js/components/`
* **Go HTML templates:** `templates/`

## General Guidelines

We recommend [Google HTML/CSS Style Guide](https://google.github.io/styleguide/htmlcssguide.html) and [Google JavaScript Style Guide](https://google.github.io/styleguide/jsguide.html)

### Gitea specific guidelines:

1. Every feature (Fomantic-UI/jQuery module) should be put in separate files/directories.
2. HTML ids and classes should use kebab-case, it's preferred to contain 2-3 feature related keywords.
3. HTML ids and classes used in JavaScript should be unique for the whole project, and should contain 2-3 feature related keywords. We recommend to use the `js-` prefix for classes that are only used in JavaScript.
4. CSS styling for classes provided by frameworks should not be overwritten. Always use new class names with 2-3 feature related keywords to overwrite framework styles. Gitea's helper CSS classes in `helpers.less` could be helpful.
5. The backend can pass complex data to the frontend by using `ctx.PageData["myModuleData"] = map[]{}`, but do not expose whole models to the frontend to avoid leaking sensitive data.
6. Simple pages and SEO-related pages use Go HTML Template render to generate static Fomantic-UI HTML output. Complex pages can use Vue3.
7. Clarify variable types, prefer `elem.disabled = true` instead of `elem.setAttribute('disabled', 'anything')`, prefer `$el.prop('checked', var === 'yes')` instead of `$el.prop('checked', var)`.
8. Use semantic elements, prefer `<button class="ui button">` instead of `<div class="ui button">`.
9. Avoid unnecessary `!important` in CSS, add comments to explain why it's necessary if it can't be avoided.
10. Avoid mixing different events in one event listener, prefer to use individual event listeners for every event.
11. Custom event names are recommended to use `ce-` prefix.
12. Gitea's tailwind-style CSS classes use `gt-` prefix (`gt-relative`), while Gitea's own private framework-level CSS classes use `g-` prefix (`g-modal-confirm`).

### Accessibility / ARIA

In history, Gitea heavily uses Fomantic UI which is not an accessibility-friendly framework.
Gitea uses some patches to make Fomantic UI more accessible (see the `aria.js` and `aria.md`),
but there are still many problems which need a lot of work and time to fix.

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

If an event listener must be `async`, the `e.preventDefault()` should be before any `await`,
it's recommended to put it at the beginning of the function.

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

### Show/Hide Elements

* Vue components are recommended to use `v-if` and `v-show` to show/hide elements.
* Go template code should use Gitea's `.gt-hidden` and `showElem()/hideElem()/toggleElem()`, see more details in `.gt-hidden`'s comment.

### Styles and Attributes in Go HTML Template

It's recommended to use:

```html
<div class="gt-name1 gt-name2 {{if .IsFoo}}gt-foo{{end}}" {{if .IsFoo}}data-foo{{end}}></div>
```

instead of:

```html
<div class="gt-name1 gt-name2{{if .IsFoo}} gt-foo{{end}}"{{if .IsFoo}} data-foo{{end}}></div>
```

to make the code more readable.

### Legacy Code

A lot of legacy code already existed before this document's written. It's recommended to refactor legacy code to follow the guidelines.

### Vue3 and JSX

Gitea is using Vue3 now. We decided not to introduce JSX to keep the HTML and the JavaScript code separated.
