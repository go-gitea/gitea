import {svg} from '../svg.ts';

const addPrefix = (str: string): string => `user-content-${str}`;
const removePrefix = (str: string): string => str.replace(/^user-content-/, '');
const hasPrefix = (str: string): boolean => str.startsWith('user-content-');

// scroll to anchor while respecting the `user-content` prefix that exists on the target
function scrollToAnchor(encodedId: string): void {
  if (!encodedId) return;
  const id = decodeURIComponent(encodedId);
  const prefixedId = addPrefix(id);
  let el = document.querySelector(`#${prefixedId}`);

  // check for matching user-generated `a[name]`
  if (!el) {
    el = document.querySelector(`a[name="${CSS.escape(prefixedId)}"]`);
  }

  // compat for links with old 'user-content-' prefixed hashes
  if (!el && hasPrefix(id)) {
    return document.querySelector(`#${id}`)?.scrollIntoView();
  }

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
      if (!href.startsWith('#user-content-')) continue;
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
