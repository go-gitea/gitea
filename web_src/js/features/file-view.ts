import {applyRenderPlugin} from '../modules/file-render-plugin.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';

/**
 * init file view renderer
 *
 * detect renderable files and apply appropriate plugins
 */
export function initFileView(): void {
  // register file view renderer init function
  registerGlobalInitFunc('initFileView', async (container: HTMLElement) => {
    // get file info
    const treePath = container.getAttribute('data-tree-path');
    const fileLink = container.getAttribute('data-raw-file-link');

    // mark loading state
    container.classList.add('is-loading');

    try {
      // check if filename and url exist
      if (!treePath || !fileLink) {
        console.error(`missing file name(${treePath}) or file url(${fileLink}) for rendering`);
        throw new Error('missing necessary file info');
      }

      // try to apply render plugin
      const success = await applyRenderPlugin(container);

      // if no suitable plugin is found, show default view
      if (!success) {
        // show default view raw file link
        const fallbackText = container.getAttribute('data-fallback-text');

        container.innerHTML = `
          <div class="view-raw-fallback">
            <a href="${fileLink}" class="ui basic button" target="_blank">${fallbackText}</a>
          </div>
        `;
      }
    } catch (error) {
      console.error('file view init error:', error);

      // show error message
      const fallbackText = container.getAttribute('data-fallback-text');
      const errorHeader = container.getAttribute('data-error-header');

      container.innerHTML = `
        <div class="ui error message">
          <div class="header">${errorHeader}</div>
          <pre>${JSON.stringify({treePath, fileLink}, null, 2)}</pre>
          <a class="ui basic button" href="${fileLink || '#'}" target="_blank">${fallbackText}</a>
        </div>
      `;
    } finally {
      // remove loading state
      container.classList.remove('is-loading');
    }
  });
}
