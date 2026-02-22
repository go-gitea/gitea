import type {FileRenderPlugin} from '../render/plugin.ts';
import {newRenderPlugin3DViewer} from '../render/plugins/3d-viewer.ts';
import {newRenderPluginPdfViewer} from '../render/plugins/pdf-viewer.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {localUserSettings} from '../modules/user-settings.ts';
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

function updateSidebarPosition(elFileView: HTMLElement, sidebar: HTMLElement): void {
  const fileHeader = elFileView.querySelector('.file-header');
  const segment = elFileView.querySelector('.ui.bottom.segment');
  if (!fileHeader || !segment) return;

  const headerRect = fileHeader.getBoundingClientRect();
  const segmentRect = segment.getBoundingClientRect();

  // Calculate top position:
  // - When file header is visible: align with file header top
  // - When file header scrolls above viewport: stick to top (12px)
  // - Ensure sidebar top doesn't go above segment top (when scrolling up)
  const minTop = 12;
  let topPos = Math.max(minTop, headerRect.top);
  topPos = Math.max(topPos, segmentRect.top); // Don't go above segment top

  // Dynamically calculate max-height so sidebar doesn't extend below segment bottom
  const availableHeight = Math.max(0, segmentRect.bottom - topPos);
  // 140px accounts for fixed layout chrome (header, spacing, etc.) and must match CSS: calc(100vh - 140px)
  const cssMaxHeightOffset = 140;
  const cssMaxHeight = window.innerHeight - cssMaxHeightOffset;
  const maxHeight = Math.min(availableHeight, cssMaxHeight);

  // Hide sidebar if available height is too small
  if (maxHeight < 50) {
    sidebar.style.opacity = '0';
    return;
  }

  sidebar.style.maxHeight = `${maxHeight}px`;
  sidebar.style.opacity = '';
  sidebar.style.top = `${topPos}px`;

  // Position sidebar right next to the content
  const leftPos = segmentRect.right + 8; // 8px gap from content
  sidebar.style.left = `${leftPos}px`;
  sidebar.style.right = 'auto';

  // Mark as positioned to show the sidebar (prevents flicker)
  sidebar.classList.add('sidebar-positioned');
}

function initSidebarToggle(elFileView: HTMLElement): void {
  const toggleBtn = elFileView.querySelector('#toggle-sidebar-btn');
  const sidebar = elFileView.querySelector<HTMLElement>('.file-view-sidebar');
  if (!toggleBtn || !sidebar) return;

  // Check if we're in file view (not home page) - only file view needs margin adjustment
  const repoViewContent = elFileView.closest<HTMLElement>('.repo-view-content');
  const isFileView = Boolean(repoViewContent);

  // Helper to update position
  const updatePosition = () => {
    if (!sidebar.classList.contains('sidebar-panel-hidden')) {
      updateSidebarPosition(elFileView, sidebar);
    }
  };

  // Helper to show sidebar with proper positioning
  const showSidebar = () => {
    toggleBtn.classList.add('active');

    // Wait for margin to take effect before showing and positioning sidebar
    const showAfterLayout = () => {
      sidebar.classList.remove('sidebar-panel-hidden');
      requestAnimationFrame(() => {
        updateSidebarPosition(elFileView, sidebar);
      });
    };

    // For file view, first add margin, wait for layout, then show sidebar
    if (isFileView && repoViewContent) {
      repoViewContent.classList.add('sidebar-visible');
      // Wait for CSS transition to actually complete before calculating position
      const onTransitionEnd = (event: TransitionEvent) => {
        if (event.target !== repoViewContent) return;
        repoViewContent.removeEventListener('transitionend', onTransitionEnd);
        showAfterLayout();
      };
      repoViewContent.addEventListener('transitionend', onTransitionEnd);
    } else {
      // For home page (README), no margin needed, show with small delay
      setTimeout(showAfterLayout, 10);
    }
  };

  // Helper to hide sidebar
  const hideSidebar = () => {
    sidebar.classList.add('sidebar-panel-hidden');
    sidebar.classList.remove('sidebar-positioned');
    toggleBtn.classList.remove('active');
    if (isFileView && repoViewContent) {
      repoViewContent.classList.remove('sidebar-visible');
    }
  };

  // Restore saved state (default to hidden)
  const isVisible = localUserSettings.getBoolean('file-view-sidebar-visible');

  // Apply initial state
  if (isVisible) {
    showSidebar();
  } else {
    hideSidebar();
  }

  // Update sidebar position on resize to keep aligned with file content
  // Only observe the segment element to avoid unnecessary updates from unrelated page changes
  const fileHeader = elFileView.querySelector('.file-header');
  const segment = elFileView.querySelector('.ui.bottom.segment');

  const resizeObserver = new ResizeObserver(() => {
    updatePosition();
  });
  if (segment) {
    resizeObserver.observe(segment);
  }

  // Update position using IntersectionObserver for scroll tracking
  // Use 101 thresholds (0%, 1%, 2%, ..., 100%) for fine-grained position updates
  if (fileHeader && segment) {
    const thresholds = Array.from({length: 101}, (_, i) => i / 100);
    const positionObserver = new IntersectionObserver((entries) => {
      // Only update if any entry is intersecting (visible)
      if (entries.some((e) => e.isIntersecting)) {
        updatePosition();
      }
    }, {threshold: thresholds});
    positionObserver.observe(segment);
    positionObserver.observe(fileHeader);
  }

  toggleBtn.addEventListener('click', () => {
    const isCurrentlyVisible = !sidebar.classList.contains('sidebar-panel-hidden');
    if (isCurrentlyVisible) {
      hideSidebar();
      localUserSettings.setBoolean('file-view-sidebar-visible', false);
    } else {
      showSidebar();
      localUserSettings.setBoolean('file-view-sidebar-visible', true);
    }
  });
}

export function initRepoFileView(): void {
  registerGlobalInitFunc('initRepoFileView', async (elFileView: HTMLElement) => {
    initPluginsOnce();

    // Initialize sidebar toggle functionality (e.g., TOC for markdown files)
    initSidebarToggle(elFileView);

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
