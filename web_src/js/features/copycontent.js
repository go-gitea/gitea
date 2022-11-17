import {copyToClipboard} from './clipboard.js';
import {showTemporaryTooltip} from '../modules/tippy.js';
const {i18n} = window.config;

export function initCopyContent() {
  const btn = document.getElementById('copy-content');
  if (!btn || btn.classList.contains('disabled')) return;

  btn.addEventListener('click', async () => {
    if (btn.classList.contains('is-loading')) return;
    let content;
    const link = btn.getAttribute('data-link');

    // when data-link is present, we perform a fetch. this is either because
    // the text to copy is not in the DOM or it is an image which should be
    // fetched to copy in full resolution
    if (link) {
      btn.classList.add('is-loading');
      try {
        const res = await fetch(link, {credentials: 'include', redirect: 'follow'});
        const contentType = res.headers.get('content-type');

        if (contentType.startsWith('image/') && !contentType.startsWith('image/svg')) {
          content = await res.blob();
        } else {
          content = await res.text();
        }
      } catch {
        return showTemporaryTooltip(btn, i18n.copy_error);
      } finally {
        btn.classList.remove('is-loading');
      }
    } else { // text, copy from DOM
      const lineEls = document.querySelectorAll('.file-view .lines-code');
      content = Array.from(lineEls).map((el) => el.textContent).join('');
    }

    try {
      const success = await copyToClipboard(content);
      showTemporaryTooltip(btn, success ? i18n.copy_success : i18n.copy_error);
    } catch {
      showTemporaryTooltip(btn, i18n.copy_error);
    }
  });
}
