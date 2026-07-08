# Frontend development guidelines

This document covers frontend-specific architecture and contribution expectations.
For the general workflow see [CONTRIBUTING.md](../CONTRIBUTING.md), and for building
and testing see [development.md](development.md) and [testing.md](testing.md).

## Background

The frontend uses [Vue 3](https://vuejs.org/), [Fomantic-UI](https://fomantic-ui.com/) (built on jQuery)
and [Tailwind CSS](https://tailwindcss.com/). Pages are rendered with Go HTML templates.
Source files live in:

- `web_src/css/`: CSS styles
- `web_src/js/`: JavaScript and TypeScript
- `web_src/js/components/`: Vue components
- `web_src/js/features/`: feature modules wired up at page load
- `templates/`: Go HTML templates

## Dependencies

Frontend dependencies are managed with [pnpm](https://pnpm.io/). The same rules as
for [backend dependencies](guidelines-backend.md#dependencies) apply, except the
relevant files are `package.json` and `pnpm-lock.yaml`, and new versions must always
reference an existing published version.

## Framework usage

Mixing frameworks arbitrarily makes code hard to maintain. Recommended combinations:

- Vue3
- Vanilla JavaScript
- Fomantic-UI (jQuery), deprecated, we vendored a specific version with a lot of changes.

Avoid combinations such as Vue with Fomantic-UI.
Vue components may reuse Fomantic-UI CSS classes for visual consistency.
Use Go templates for simple or SEO-relevant pages and Vue for complex, interactive pages.
Gitea uses Vue 3 **without** JSX to keep HTML and JavaScript separate.

> [!NOTE]
> Fomantic-UI is not an accessibility-friendly framework. Gitea patches some ARIA
> behavior, but accessibility work is ongoing — prefer semantic HTML and test
> keyboard/screen-reader behavior where you can.

## Gitea-specific conventions

- Keep features in their own files or directories.
- Use kebab-case for HTML `id`s and classes, ideally with 2-3 feature keywords.
- Prefix classes to avoid short-name conflicts between different frameworks.
- Create a new class name when overriding framework styles instead of editing the framework's own classes,
  or fix the framework's source to fix all cases.
- Prefer semantic elements such as `<button>` over generic `<div>`s.
- Avoid `!important`; when it is unavoidable, document why.
- Prefix custom DOM events with `ce-`.

## CSS

Prefer Tailwind utility classes with the `tw-` prefix, and the `flex-*` layout
helpers over per-child margins. Gitea also ships a small set of custom helpers:
`gt-` for general helpers and `g-` for framework-level helpers (see
`web_src/css/helpers.css`); use these only when a Tailwind utility does not exist.

Write class attributes as a single readable unit in templates:

```html
<div class="flex-text-inline {{if .IsFoo}}tw-hidden{{end}}"></div>
```

## TypeScript

- Use `import type` for type-only imports.
- Prefer `@ts-expect-error` over `@ts-ignore`.
- Use the `!` non-null assertion (rather than `?.`/`??`) when a value is known to always exist.
- Only mark a function `async` when it actually uses `await` or returns a `Promise`.
  Avoid async event listeners; if unavoidable, call `e.preventDefault()` before the
  first `await`. For a deliberately un-awaited call, assign it: `const _promise = asyncFoo()`.

## Data fetching

Use the `GET`, `POST`, `PUT`, `PATCH`, and `DELETE` wrappers from
[`web_src/js/modules/fetch.ts`](../web_src/js/modules/fetch.ts).

## DOM attributes

Avoid `node.dataset` because of its camel-casing behavior; use `node.getAttribute`
in new code. Never bind user-provided data directly onto DOM nodes.

## Showing and hiding elements

- In Vue, use `v-if` and `v-show`.
- In Go templates and plain JavaScript, use the `.tw-hidden` class together with the
  `showElem()`, `hideElem()`, and `toggleElem()` helpers from
  [`web_src/js/utils/dom.ts`](../web_src/js/utils/dom.ts).

## UI component gallery

When running Gitea in development mode, standardized UI components are available at
`/devtest` (for example `http://localhost:3000/devtest`). These pages are also used
by the e2e tests.
