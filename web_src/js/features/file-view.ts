import type {FileRenderPlugin} from '../render/plugin.ts';
import {newRenderPlugin3DViewer} from '../render/plugins/3d-viewer.ts';
import {newRenderPluginPdfViewer} from '../render/plugins/pdf-viewer.ts';
import {loadDynamicRenderPlugins} from '../render/plugins/dynamic-plugin.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {createElementFromHTML, showElem, toggleElemClass} from '../utils/dom.ts';
import {html} from '../utils/html.ts';
import {basename} from '../utils.ts';

const plugins: FileRenderPlugin[] = [];
let pluginsInitialized = false;
let pluginsInitPromise: Promise<void> | null = null;

export function decodeHeadChunk(value: string | null): Uint8Array | null {
  if (!value) return null;
  try {
    const binary = window.atob(value);
    const buffer = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
      buffer[i] = binary.charCodeAt(i);
    }
    return buffer;
  } catch (err) {
    console.error('Failed to decode render plugin head chunk', err);
    return null;
  }
}

async function initPluginsOnce(): Promise<void> {
  if (pluginsInitialized) return;
  if (!pluginsInitPromise) {
    pluginsInitPromise = (async () => {
      if (!pluginsInitialized) {
        plugins.push(newRenderPlugin3DViewer(), newRenderPluginPdfViewer());
        const dynamicPlugins = await loadDynamicRenderPlugins();
        plugins.push(...dynamicPlugins);
        pluginsInitialized = true;
      }
    })();
  }
  await pluginsInitPromise;
}

function findFileRenderPlugin(filename: string, mimeType: string, headChunk: Uint8Array | null): FileRenderPlugin | null {
  return plugins.find((plugin) => plugin.canHandle(filename, mimeType, headChunk)) || null;
}

function showRenderRawFileButton(elFileView: HTMLElement, renderContainer: HTMLElement | null): void {
  const toggleButtons = elFileView.querySelector('.file-view-toggle-buttons')!;
  showElem(toggleButtons);
  const displayingRendered = Boolean(renderContainer);
  toggleElemClass(toggleButtons.querySelectorAll('.file-view-toggle-source'), 'active', !displayingRendered); // it may not exist
  toggleElemClass(toggleButtons.querySelector('.file-view-toggle-rendered')!, 'active', displayingRendered);
  // TODO: if there is only one button, hide it?
}

async function renderRawFileToContainer(container: HTMLElement, rawFileLink: string, mimeType: string, headChunk: Uint8Array | null) {
  const elViewRawPrompt = container.querySelector('.file-view-raw-prompt');
  if (!rawFileLink || !elViewRawPrompt) throw new Error('unexpected file view container');

  let rendered = false, errorMsg = '';
  try {
    const plugin = findFileRenderPlugin(basename(rawFileLink), mimeType, headChunk);
    if (plugin) {
      container.classList.add('is-loading');
      container.setAttribute('data-render-name', plugin.name); // not used yet
      await plugin.render(container, rawFileLink, {mimeType, headChunk});
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
    const elErrorMessage = createElementFromHTML(html`<div class="ui error message">${errorMsg}</div>`);
    elViewRawPrompt.insertAdjacentElement('afterbegin', elErrorMessage);
  }
}

export function initRepoFileView(): void {
  registerGlobalInitFunc('initRepoFileView', async (elFileView: HTMLElement) => {
    await initPluginsOnce();
    const rawFileLink = elFileView.getAttribute('data-raw-file-link')!;
    const mimeType = elFileView.getAttribute('data-mime-type') || '';
    const headChunk = decodeHeadChunk(elFileView.getAttribute('data-head-chunk'));
    const plugin = findFileRenderPlugin(basename(rawFileLink), mimeType, headChunk);
    if (!plugin) return;

    const renderContainer = elFileView.querySelector<HTMLElement>('.file-view-render-container');
    showRenderRawFileButton(elFileView, renderContainer);
    // maybe in the future multiple plugins can render the same file, so we should not assume only one plugin will render it
    if (renderContainer) await renderRawFileToContainer(renderContainer, rawFileLink, mimeType, headChunk);
  });
}
