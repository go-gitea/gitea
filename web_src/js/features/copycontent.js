import {clippie} from 'clippie';
import {showTemporaryTooltip} from '../modules/tippy.js';
import {convertImage} from '../utils.js';
import {GET} from '../modules/fetch.js';

const {i18n} = window.config;

export function initCopyContent() {
  const btn = document.querySelector('#copy-content');
  if (!btn || btn.classList.contains('disabled')) return;

  btn.addEventListener('click', async () => {
    if (btn.classList.contains('is-loading')) return;
    let content;
    let isRasterImage = false;
    const link = btn.getAttribute('data-link');

    // when data-link is present, we perform a fetch. this is either because
    // the text to copy is not in the DOM or it is an image which should be
    // fetched to copy in full resolution
    if (link) {
      btn.classList.add('is-loading', 'loading-icon-2px');
      try {
        const res = await GET(link, {credentials: 'include', redirect: 'follow'});
        const contentType = res.headers.get('content-type');

        if (contentType.startsWith('image/') && !contentType.startsWith('image/svg')) {
          isRasterImage = true;
          content = await res.blob();
        } else {
          content = await res.text();
        }
      } catch {
        return showTemporaryTooltip(btn, i18n.copy_error);
      } finally {
        btn.classList.remove('is-loading', 'loading-icon-2px');
      }
    } else { // text, read from DOM
      const lineEls = document.querySelectorAll('.file-view .lines-code');
      content = Array.from(lineEls, (el) => el.textContent).join('');
    }

    // try copy original first, if that fails and it's an image, convert it to png
    const success = await clippie(content);
    if (success) {
      showTemporaryTooltip(btn, i18n.copy_success);
    } else {
      if (isRasterImage) {
        const success = await clippie(await convertImage(content, 'image/png'));
        showTemporaryTooltip(btn, success ? i18n.copy_success : i18n.copy_error);
      } else {
        showTemporaryTooltip(btn, i18n.copy_error);
      }
    }
  });
}
