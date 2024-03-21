import {svg} from '../svg.js';

// scroll to anchor while respecting the `user-content` prefix that exists on the target
function scrollToAnchor(encodedId, initial) {
  // abort if the browser has already scrolled to another anchor during page load
  if (!encodedId || (initial && document.querySelector(':target'))) return;
  const id = decodeURIComponent(encodedId);
  let el = document.getElementById(`user-content-${id}`);

  // check for matching user-generated `a[name]`
  if (!el) {
    const nameAnchors = document.getElementsByName(`user-content-${id}`);
    if (nameAnchors.length) {
      el = nameAnchors[0];
    }
  }

  // compat for links with old 'user-content-' prefixed hashes
  if (!el && id.startsWith('user-content-')) {
    const el = document.getElementById(id);
    if (el) el.scrollIntoView();
  }

  if (el) {
    el.scrollIntoView();
  }
}

export function initMarkupAnchors() {
  const markupEls = document.querySelectorAll('.markup');
  if (!markupEls.length) return;

  for (const markupEl of markupEls) {
    // create link icons for markup headings, the resulting link href will remove `user-content-`
    for (const heading of markupEl.querySelectorAll(`:is(h1, h2, h3, h4, h5, h6`)) {
      const originalId = heading.id.replace(/^user-content-/, '');
      const a = document.createElement('a');
      a.classList.add('anchor');
      a.setAttribute('href', `#${encodeURIComponent(originalId)}`);
      a.innerHTML = svg('octicon-link');
      heading.prepend(a);
    }

    // remove `user-content-` prefix from links so they don't show in url bar when clicked
    for (const a of markupEl.querySelectorAll('a[href^="#"]')) {
      const href = a.getAttribute('href');
      if (!href.startsWith('#user-content-')) continue;
      const originalId = href.replace(/^#user-content-/, '');
      a.setAttribute('href', `#${originalId}`);
    }

    // add `user-content-` prefix to user-generated `a[name]` link targets
    // TODO: this prefix should be added in backend instead
    for (const a of markupEl.querySelectorAll('a[name]')) {
      const name = a.getAttribute('name');
      if (!name) continue;
      a.setAttribute('name', `user-content-${a.name}`);
    }

    for (const a of markupEl.querySelectorAll('a[href^="#"]')) {
      a.addEventListener('click', (e) => {
        scrollToAnchor(e.currentTarget.getAttribute('href')?.substring(1), false);
      });
    }
  }

  scrollToAnchor(window.location.hash.substring(1), true);
}
