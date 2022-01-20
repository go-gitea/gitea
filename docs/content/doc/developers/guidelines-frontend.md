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


### Framework Usage

Mixing different frameworks together is highly discouraged. A JavaScript module should follow one major framework and follow the framework's best practice.

Recommended implementations:
* Vue + Native
* Fomantic-UI (jQuery)
* Native only

Discouraged implementations:
* Vue + jQuery
* jQuery + Native

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

#### DOM Event Listener

```js
el.addEventListener('click', (e) => {
  (async () => {
    await asyncFoo(); // recommended
    // then we shound't do e.preventDefault() after await, no effect
  })(); 
  
  const _promise = asyncFoo(); // recommended

  e.preventDefault(); // correct
});

el.addEventListener('async', async (e) => { // not recommended but acceptable
  e.preventDefault(); // acceptable
  await asyncFoo();   // skip out event dispatch
  e.preventDefault(); // WRONG
});
```

#### jQuery Event Listener

```js
$('#el').on('click', (e) => {
  (async () => {
    await asyncFoo(); // recommended
    // then we shound't do e.preventDefault() after await, no effect
  })();

  const _promise = asyncFoo(); // recommended

  e.preventDefault();  // correct
  return false;        // correct
});

$('#el').on('click', async (e) => {  // not recommended but acceptable
  e.preventDefault();  // acceptable
  return false;        // WRONG, jQuery expects the returned value is a boolean, not a Promise
  await asyncFoo();    // skip out event dispatch
  return false;        // WRONG
});
```

### HTML Attributes and `dataset`

We forbid `dataset` usage, its camel-casing behaviour makes it hard to grep for attributes. However there are still some special cases, so the current guideline is:

* For legacy code:
  * `$.data()` should be refactored to `$.attr()`.
  * `$.data()` can be used to bind some non-string data to elements in rare cases, but it is highly discouraged.

* For new code:
  * `node.dataset` should not be used, use `node.getAttribute` instead. 
  * never bind any user data to a DOM node, use a suitable design pattern to describe the relation between node and data.


### Vue2/Vue3 and JSX

Gitea is using Vue2 now, we plan to upgrade to Vue3. We decided not to introduce JSX to keep the HTML and the JavaScript code separated.
