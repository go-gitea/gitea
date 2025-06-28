import {applyRenderPlugin} from '../modules/file-render-plugin.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {createElementFromHTML} from '../utils/dom.ts';
import {register3DViewerPlugin} from '../render/plugins/3d-viewer.ts';
import {registerPdfViewerPlugin} from '../render/plugins/pdf-viewer.ts';
import {htmlEscape} from 'escape-goat';

export function initFileViewRender(): void {
  let pluginRegistered = false;

  registerGlobalInitFunc('initFileViewRender', async (container: HTMLElement) => {
    if (!pluginRegistered) {
      pluginRegistered = true;
      register3DViewerPlugin();
      registerPdfViewerPlugin();
    }

    const rawFileLink = container.getAttribute('data-raw-file-link');
    const elViewRawPrompt = container.querySelector('.file-view-raw-prompt');
    if (!rawFileLink || !elViewRawPrompt) throw new Error('unexpected file view container');

    let rendered = false, errorMsg = '';
    try {
      rendered = await applyRenderPlugin(container, rawFileLink);
    } catch (e) {
      errorMsg = `${e}`;
    }

    if (rendered) {
      elViewRawPrompt.remove();
      return;
    }

    // remove all children from the container, and only show the raw file link
    container.replaceChildren(elViewRawPrompt);

    if (errorMsg) {
      const elErrorMessage = createElementFromHTML(htmlEscape`<div class="ui error message">${errorMsg}</div>`);
      elViewRawPrompt.insertAdjacentElement('afterbegin', elErrorMessage);
    }
  });
}
