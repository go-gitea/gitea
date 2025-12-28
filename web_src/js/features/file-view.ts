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

function updateTocPosition(elFileView: HTMLElement, tocSidebar: HTMLElement): void {
  const fileHeader = elFileView.querySelector('.file-header');
  if (!fileHeader) return;

  const headerRect = fileHeader.getBoundingClientRect();
  // Align TOC top with the file header top, with a minimum of 12px from viewport top
  const topPos = Math.max(12, headerRect.top);
  tocSidebar.style.top = `${topPos}px`;

  // Position TOC right next to the content (works for both file view and home page)
  const segment = elFileView.querySelector('.ui.segment');
  if (segment) {
    const segmentRect = segment.getBoundingClientRect();
    const leftPos = segmentRect.right + 8; // 8px gap from content
    tocSidebar.style.left = `${leftPos}px`;
    tocSidebar.style.right = 'auto';
  }

  // Mark as positioned to show the TOC (prevents flicker)
  tocSidebar.classList.add('toc-positioned');
}

function initTocToggle(elFileView: HTMLElement): void {
  const toggleBtn = elFileView.querySelector('#toggle-toc-btn');
  const tocSidebar = elFileView.querySelector<HTMLElement>('.file-view-toc');
  if (!toggleBtn || !tocSidebar) return;

  // Check if we're in file view (not home page) - only file view needs margin adjustment
  const repoViewContent = elFileView.closest('.repo-view-content');
  const isFileView = Boolean(repoViewContent);

  // Helper to update position
  const updatePosition = () => {
    if (!tocSidebar.classList.contains('toc-panel-hidden')) {
      updateTocPosition(elFileView, tocSidebar);
    }
  };

  // Helper to show TOC with proper positioning
  const showToc = () => {
    toggleBtn.classList.add('active');

    // Wait for margin to take effect before showing and positioning TOC
    const showAfterLayout = () => {
      tocSidebar.classList.remove('toc-panel-hidden');
      requestAnimationFrame(() => {
        updateTocPosition(elFileView, tocSidebar);
      });
    };

    // For file view, first add margin, wait for layout, then show TOC
    if (isFileView) {
      repoViewContent.classList.add('toc-visible');
      // Wait for CSS transition to complete (200ms) before calculating position
      setTimeout(showAfterLayout, 220);
    } else {
      // For home page (README), no margin needed, show with small delay
      setTimeout(showAfterLayout, 10);
    }
  };

  // Helper to hide TOC
  const hideToc = () => {
    tocSidebar.classList.add('toc-panel-hidden');
    tocSidebar.classList.remove('toc-positioned');
    toggleBtn.classList.remove('active');
    if (isFileView) {
      repoViewContent.classList.remove('toc-visible');
    }
  };

  // Restore saved state from localStorage (default to hidden)
  const savedState = localStorage.getItem('file-view-toc-visible');
  const isVisible = savedState === 'true'; // default to hidden

  // Apply initial state
  if (isVisible) {
    showToc();
  } else {
    hideToc();
  }

  // Update TOC position on resize/scroll to keep aligned with file content
  const resizeObserver = new ResizeObserver(() => {
    updatePosition();
  });
  resizeObserver.observe(document.body);

  const fileHeader = elFileView.querySelector('.file-header');
  if (fileHeader) {
    const intersectionObserver = new IntersectionObserver(() => {
      updatePosition();
    }, {
      root: null,
      rootMargin: '0px',
      threshold: [0, 0.25, 0.5, 0.75, 1.0],
    });
    intersectionObserver.observe(fileHeader);
  }

  toggleBtn.addEventListener('click', () => {
    const isCurrentlyVisible = !tocSidebar.classList.contains('toc-panel-hidden');
    if (isCurrentlyVisible) {
      hideToc();
      localStorage.setItem('file-view-toc-visible', 'false');
    } else {
      showToc();
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
