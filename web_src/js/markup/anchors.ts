import {svg} from '../svg.ts';

// Rendered content from users have IDs prefixed with `user-content-` to avoid conflicts with other IDs on the page.
// - security concern: elements with IDs can affect frontend logic, for example: sending requests.
// To make end users have better experience, the prefixes are stripped from the href attributes of links.
// The same as GitHub: backend generates anchor `id="user-content-faq"` but the link shown to users is `href="#faq"`.
//
// At the moment, the anchor processing works like this:
// - backend adds `user-content-` prefix for elements like `<h1 id>` and `<a href>`
// - js adds the `user-content-` prefix to user-generated `<a name>` targets
// - js intercepts the hash navigation on page load and whenever a link is clicked
//   to add the prefix so the correct prefixed `id`/`name` element is focused
//
// TODO: ideally, backend should be able to generate elements with necessary anchors,
// backend doesn't need to add the prefix to `href`, then frontend doesn't need to spend
// time on adding new elements or removing the prefixes.

const addPrefix = (str: string): string => `user-content-${str}`;
const removePrefix = (str: string): string => str.replace(/^user-content-/, '');
const hasPrefix = (str: string): boolean => str.startsWith('user-content-');

// scroll to anchor while respecting the `user-content` prefix that exists on the target
function scrollToAnchor(encodedId?: string): void {
  // FIXME: need to rewrite this function with new a better markup anchor generation logic, too many tricks here
  let elemId: string | undefined;
  try {
    elemId = decodeURIComponent(encodedId ?? '');
  } catch {} // ignore the errors, since the "encodedId" is from user's input
  if (!elemId) return;

  const prefixedId = addPrefix(elemId);
  // eslint-disable-next-line unicorn/prefer-query-selector
  let el = document.getElementById(prefixedId);

  // check for matching user-generated `a[name]`
  el = el ?? document.querySelector(`a[name="${CSS.escape(prefixedId)}"]`);

  // compat for links with old 'user-content-' prefixed hashes
  // eslint-disable-next-line unicorn/prefer-query-selector
  el = (!el && hasPrefix(elemId)) ? document.getElementById(elemId) : el;

  el?.scrollIntoView();
}

export function initMarkupAnchors(): void {
  const markupEls = document.querySelectorAll('.markup');
  if (!markupEls.length) return;

  for (const markupEl of markupEls) {
    // create link icons for markup headings, the resulting link href will remove `user-content-`
    for (const heading of markupEl.querySelectorAll('h1, h2, h3, h4, h5, h6')) {
      const a = document.createElement('a');
      a.classList.add('anchor');
      a.setAttribute('href', `#${encodeURIComponent(removePrefix(heading.id))}`);
      a.innerHTML = svg('octicon-link');
      heading.prepend(a);
    }

    // remove `user-content-` prefix from links so they don't show in url bar when clicked
    for (const a of markupEl.querySelectorAll<HTMLAnchorElement>('a[href^="#"]')) {
      const href = a.getAttribute('href');
      if (!href?.startsWith('#user-content-')) continue;
      a.setAttribute('href', `#${removePrefix(href.substring(1))}`);
    }

    // add `user-content-` prefix to user-generated `a[name]` link targets
    // TODO: this prefix should be added in backend instead
    for (const a of markupEl.querySelectorAll<HTMLAnchorElement>('a[name]')) {
      const name = a.getAttribute('name');
      if (!name) continue;
      a.setAttribute('name', addPrefix(name));
    }

    for (const a of markupEl.querySelectorAll<HTMLAnchorElement>('a[href^="#"]')) {
      a.addEventListener('click', (e) => {
        scrollToAnchor((e.currentTarget as HTMLAnchorElement).getAttribute('href')?.substring(1));
      });
    }
  }

  // scroll to anchor unless the browser has already scrolled somewhere during page load
  if (!document.querySelector(':target')) {
    scrollToAnchor(window.location.hash?.substring(1));
  }
}
