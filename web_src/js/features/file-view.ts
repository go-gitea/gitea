import type {FileRenderPlugin} from '../render/plugin.ts';
import {newRenderPlugin3DViewer} from '../render/plugins/3d-viewer.ts';
import {newRenderPluginPdfViewer} from '../render/plugins/pdf-viewer.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {createElementFromHTML, showElem, toggleElemClass} from '../utils/dom.ts';
import {html} from '../utils/html.ts';
import {basename} from '../utils.ts';

const plugins: FileRenderPlugin[] = [];

function initPluginsOnce(): void {
  if (plugins.length) return;
  plugins.push(newRenderPlugin3DViewer(), newRenderPluginPdfViewer());
}

function findFileRenderPlugin(filename: string, mimeType: string): FileRenderPlugin | null {
  return plugins.find((plugin) => plugin.canHandle(filename, mimeType)) || null;
}

function showRenderRawFileButton(elFileView: HTMLElement, renderContainer: HTMLElement | null): void {
  const toggleButtons = elFileView.querySelector('.file-view-toggle-buttons')!;
  showElem(toggleButtons);
  const displayingRendered = Boolean(renderContainer);
  toggleElemClass(toggleButtons.querySelectorAll('.file-view-toggle-source'), 'active', !displayingRendered); // it may not exist
  toggleElemClass(toggleButtons.querySelector('.file-view-toggle-rendered')!, 'active', displayingRendered);
  // TODO: if there is only one button, hide it?
}

async function renderRawFileToContainer(container: HTMLElement, rawFileLink: string, mimeType: string) {
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
    const elErrorMessage = createElementFromHTML(html`<div class="ui error message">${errorMsg}</div>`);
    elViewRawPrompt.insertAdjacentElement('afterbegin', elErrorMessage);
  }
}

function initTocToggle(elFileView: HTMLElement): void {
  const toggleBtn = elFileView.querySelector('#toggle-toc-btn');
  const tocSidebar = elFileView.querySelector('.file-view-toc');
  if (!toggleBtn || !tocSidebar) return;

  // Restore saved state from localStorage (default to hidden)
  const savedState = localStorage.getItem('file-view-toc-visible');
  const isVisible = savedState === 'true'; // default to hidden

  // Apply initial state
  if (isVisible) {
    tocSidebar.classList.remove('toc-panel-hidden');
    toggleBtn.classList.add('active');
  } else {
    tocSidebar.classList.add('toc-panel-hidden');
    toggleBtn.classList.remove('active');
  }

  toggleBtn.addEventListener('click', () => {
    const isCurrentlyVisible = !tocSidebar.classList.contains('toc-panel-hidden');
    if (isCurrentlyVisible) {
      // Hide TOC
      tocSidebar.classList.add('toc-panel-hidden');
      toggleBtn.classList.remove('active');
      localStorage.setItem('file-view-toc-visible', 'false');
    } else {
      // Show TOC
      tocSidebar.classList.remove('toc-panel-hidden');
      toggleBtn.classList.add('active');
      localStorage.setItem('file-view-toc-visible', 'true');
    }
  });
}

export function initRepoFileView(): void {
  registerGlobalInitFunc('initRepoFileView', async (elFileView: HTMLElement) => {
    initPluginsOnce();

    // Initialize TOC toggle functionality
    initTocToggle(elFileView);

    const rawFileLink = elFileView.getAttribute('data-raw-file-link')!;
    const mimeType = elFileView.getAttribute('data-mime-type') || ''; // not used yet
    // TODO: we should also provide the prefetched file head bytes to let the plugin decide whether to render or not
    const plugin = findFileRenderPlugin(basename(rawFileLink), mimeType);
    if (!plugin) return;

    const renderContainer = elFileView.querySelector<HTMLElement>('.file-view-render-container');
    showRenderRawFileButton(elFileView, renderContainer);
    // maybe in the future multiple plugins can render the same file, so we should not assume only one plugin will render it
    if (renderContainer) await renderRawFileToContainer(renderContainer, rawFileLink, mimeType);
  });
}
