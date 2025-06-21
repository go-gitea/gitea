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
    const filename = container.getAttribute('data-filename');
    const fileUrl = container.getAttribute('data-url');

    // mark loading state
    container.classList.add('is-loading');

    try {
      // check if filename and url exist
      if (!filename || !fileUrl) {
        console.error(`missing filename(${filename}) or file url(${fileUrl}) for rendering`);
        throw new Error('missing necessary file info');
      }

      // try to apply render plugin
      const success = await applyRenderPlugin(container);

      // if no suitable plugin is found, show default view
      if (!success) {
        // show default view raw file link
        const fallbackText = container.getAttribute('data-fallback-text') || 'View Raw File';

        container.innerHTML = `
          <div class="view-raw-fallback">
            <a href="${fileUrl}" class="ui basic button" target="_blank">${fallbackText}</a>
          </div>
        `;
      }
    } catch (error) {
      console.error('file view init error:', error);

      // show error message
      const fallbackText = container.getAttribute('data-fallback-text') || 'View Raw File';

      container.innerHTML = `
        <div class="ui error message">
          <div class="header">Failed to render file</div>
          <p>Error: ${String(error)}</p>
          <pre>${JSON.stringify({filename, fileUrl}, null, 2)}</pre>
          <a class="ui basic button" href="${fileUrl || '#'}" target="_blank">${fallbackText}</a>
        </div>
      `;
    } finally {
      // remove loading state
      container.classList.remove('is-loading');
    }
  });
}
