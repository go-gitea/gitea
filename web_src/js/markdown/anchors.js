import {svg} from '../svg.js';

const headingSelector = '.markdown h1, .markdown h2, .markdown h3, .markdown h4, .markdown h5, .markdown h6';

function scrollToAnchor() {
  if (document.querySelector(':target')) return;
  if (!window.location.hash || window.location.hash.length <= 1) return;
  const id = decodeURIComponent(window.location.hash.substring(1));
  const el = document.getElementById(`user-content-${id}`);
  if (el) {
    el.scrollIntoView();
  } else if (id.startsWith('user-content-')) { // compat for links with old 'user-content-' prefixed hashes
    const el = document.getElementById(id);
    if (el) el.scrollIntoView();
  }
}

export default function initMarkdownAnchors() {
  if (!document.querySelector('.markdown')) return;

  for (const heading of document.querySelectorAll(headingSelector)) {
    const originalId = heading.id.replace(/^user-content-/, '');
    const a = document.createElement('a');
    a.classList.add('anchor');
    a.setAttribute('href', `#${encodeURIComponent(originalId)}`);
    a.innerHTML = svg('octicon-link');
    heading.prepend(a);
  }

  scrollToAnchor();
  window.addEventListener('hashchange', scrollToAnchor);
}
