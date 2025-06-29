import type {FileRenderPlugin} from '../render/plugin.ts';
import {newRenderPlugin3DViewer} from '../render/plugins/3d-viewer.ts';
import {newRenderPluginPdfViewer} from '../render/plugins/pdf-viewer.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {createElementFromHTML} from '../utils/dom.ts';
import {htmlEscape} from 'escape-goat';
import {basename} from '../utils.ts';

const plugins: FileRenderPlugin[] = [];

function initPluginsOnce(): void {
  if (plugins.length) return;
  plugins.push(newRenderPlugin3DViewer(), newRenderPluginPdfViewer());
}

function findFileRenderPlugin(filename: string, mimeType: string): FileRenderPlugin | null {
  return plugins.find((plugin) => plugin.canHandle(filename, mimeType)) || null;
}

export function initFileViewRender(): void {
  registerGlobalInitFunc('initFileViewRender', async (container: HTMLElement) => {
    initPluginsOnce();

    const rawFileLink = container.getAttribute('data-raw-file-link');
    const mimeType = container.getAttribute('data-mime-type') || ''; // not used yet
    const elViewRawPrompt = container.querySelector('.file-view-raw-prompt');
    if (!rawFileLink || !elViewRawPrompt) throw new Error('unexpected file view container');

    let rendered = false, errorMsg = '';
    try {
      const plugin = findFileRenderPlugin(basename(rawFileLink), mimeType);
      if (plugin) {
        container.classList.add('is-loading');
        container.setAttribute('data-render-name', plugin.name); // not used yet
        await plugin.render(container, rawFileLink);
        rendered = true;
      }
    } catch (e) {
      errorMsg = `${e}`;
    } finally {
      container.classList.remove('is-loading');
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
