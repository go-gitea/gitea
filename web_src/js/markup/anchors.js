import {svg} from '../svg.js';

const headingSelector = '.markup h1, .markup h2, .markup h3, .markup h4, .markup h5, .markup h6';

function scrollToAnchor(hash, initial) {
  // abort if the browser has already scrolled to another anchor during page load
  if (initial && document.querySelector(':target')) return;
  if (hash?.length <= 1) return;
  const id = decodeURIComponent(hash.substring(1));
  const el = document.getElementById(`user-content-${id}`);
  if (el) {
    el.scrollIntoView();
  } else if (id.startsWith('user-content-')) { // compat for links with old 'user-content-' prefixed hashes
    const el = document.getElementById(id);
    if (el) el.scrollIntoView();
  }
}

export function initMarkupAnchors() {
  if (!document.querySelector('.markup')) return;

  for (const heading of document.querySelectorAll(headingSelector)) {
    const originalId = heading.id.replace(/^user-content-/, '');
    const a = document.createElement('a');
    a.classList.add('anchor');
    a.setAttribute('href', `#${encodeURIComponent(originalId)}`);
    a.innerHTML = svg('octicon-link');
    a.addEventListener('click', (e) => {
      scrollToAnchor(e.currentTarget.getAttribute('href'), false);
    });
    heading.prepend(a);
  }

  scrollToAnchor(window.location.hash, true);
}
