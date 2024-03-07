import {svg} from '../svg.js';

const headingSelector = '.markup h1, .markup h2, .markup h3, .markup h4, .markup h5, .markup h6';

// scroll to anchor while respecting the `user-content` prefix that exists on the target
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

  // create link icons for markup headings, the resulting link href will remove `user-content-`
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

  // handle user-defined `name` anchors like `[Link](#link)` linking to `<a name="link"></a>Link`
  for (const a of document.querySelectorAll('.markup a[href^="#"]')) {
    const href = a.getAttribute('href');
    if (!href.startsWith('#user-content-')) continue;
    const originalId = href.replace(/^#user-content-/, '');
    a.setAttribute('href', `#${encodeURIComponent(originalId)}`);
    if (document.getElementsByName(originalId).length !== 1) {
      a.addEventListener('click', (e) => {
        scrollToAnchor(e.currentTarget.getAttribute('href'), false);
      });
    }
  }

  scrollToAnchor(window.location.hash, true);
}
