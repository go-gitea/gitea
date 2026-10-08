import {copyToClipboardWithFeedback} from '../modules/clipboard.ts';
import {convertImage} from '../utils.ts';
import {GET} from '../modules/fetch.ts';
import {registerGlobalEventFunc} from '../modules/observer.ts';

export function initCopyContent() {
  registerGlobalEventFunc('click', 'onCopyContentButtonClick', async (btn: HTMLElement) => {
    if (btn.classList.contains('disabled') || btn.classList.contains('is-loading')) return;
    await copyToClipboardWithFeedback(btn, async () => {
      const rawFileLink = btn.getAttribute('data-raw-file-link');
      if (!rawFileLink) {
        const lineEls = document.querySelectorAll('.file-view .lines-code');
        return Array.from(lineEls, (el) => el.textContent).join('');
      }
      const res = await GET(rawFileLink, {credentials: 'include', redirect: 'follow'});
      const contentType = res.headers.get('content-type')!;
      if (contentType.startsWith('image/') && !contentType.startsWith('image/svg')) {
        // browsers only accept image/png in the clipboard, convert other raster formats
        const blob = await res.blob();
        return contentType === 'image/png' ? blob : convertImage(blob, 'image/png');
      }
      return await res.text();
    });
  });
}
