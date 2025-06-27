import type {FileRenderPlugin} from '../../modules/file-render-plugin.ts';
import {registerFileRenderPlugin} from '../../modules/file-render-plugin.ts';

export function registerPdfViewerPlugin(): void {
  const plugin: FileRenderPlugin = {
    name: 'pdf-viewer',
    canHandle(filename: string, _mimeType: string): boolean {
      return filename.toLowerCase().endsWith('.pdf');
    },
    async render(container: HTMLElement, fileUrl: string): Promise<void> {
      try {
        const PDFObject = await import(/* webpackChunkName: "pdfobject" */'pdfobject');
        container.classList.add('pdf-view-content');
        const fallbackText = container.getAttribute('data-fallback-text');
        PDFObject.default.embed(fileUrl, container, {
          fallbackLink: `<a role="button" class="ui basic button pdf-fallback-button" href="${fileUrl}" target="_blank">${fallbackText}</a>`,
        });
      } catch (error) {
        console.error('error rendering PDF:', error);
        throw error;
      }
    },
  };
  registerFileRenderPlugin(plugin);
}
