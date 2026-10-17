import type {InplaceRenderPlugin} from '../render/plugin.ts';
import {newInplacePluginPdfViewer} from '../render/plugins/inplace-pdf-viewer.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {createElementFromHTML} from '../utils/dom.ts';
import {errorMessage} from '../modules/errors.ts';
import {html} from '../utils/html.ts';
import {basename} from '../utils.ts';

const inplacePlugins: InplaceRenderPlugin[] = [];

function initInplacePluginsOnce(): void {
  if (inplacePlugins.length) return;
  inplacePlugins.push(newInplacePluginPdfViewer());
}

function findInplaceRenderPlugin(filename: string, mimeType: string): InplaceRenderPlugin | null {
  return inplacePlugins.find((plugin) => plugin.canHandle(filename, mimeType)) || null;
}

async function renderRawFileToContainer(container: HTMLElement, rawFileLink: string, mimeType: string) {
  const elViewRawPrompt = container.querySelector('.file-view-raw-prompt');
  if (!rawFileLink || !elViewRawPrompt) throw new Error('unexpected file view container');

  let rendered = false, errorMsg = '';
  try {
    const plugin = findInplaceRenderPlugin(basename(rawFileLink), mimeType);
    if (plugin) {
      container.classList.add('is-loading');
      container.setAttribute('data-render-name', plugin.name); // not used yet
      await plugin.render(container, rawFileLink);
      rendered = true;
    }
  } catch (e) {
    errorMsg = errorMessage(e);
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
    const elErrorMessage = createElementFromHTML(html`<div class="ui error message">${errorMsg}</div>`);
    elViewRawPrompt.insertAdjacentElement('afterbegin', elErrorMessage);
  }
}

export function initRepoFileView(): void {
  registerGlobalInitFunc('initRepoFileView', async (elFileView: HTMLElement) => {
    initInplacePluginsOnce();
    const rawFileLink = elFileView.getAttribute('data-raw-file-link')!;
    const mimeType = elFileView.getAttribute('data-mime-type') || ''; // not used yet
    const plugin = findInplaceRenderPlugin(basename(rawFileLink), mimeType);
    if (!plugin) return;

    const renderContainer = elFileView.querySelector<HTMLElement>('.file-view-render-container');
    if (renderContainer) await renderRawFileToContainer(renderContainer, rawFileLink, mimeType);
  });
}
