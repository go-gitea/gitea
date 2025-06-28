import type {FileRenderPlugin} from '../../modules/file-render-plugin.ts';
import {registerFileRenderPlugin} from '../../modules/file-render-plugin.ts';

export function registerPdfViewerPlugin(): void {
  const plugin: FileRenderPlugin = {
    name: 'pdf-viewer',
    canHandle(filename: string, _mimeType: string): boolean {
      return filename.toLowerCase().endsWith('.pdf');
    },
    async render(container: HTMLElement, fileUrl: string): Promise<void> {
      const PDFObject = await import(/* webpackChunkName: "pdfobject" */'pdfobject');
      // TODO: the PDFObject library does not support dynamic height adjustment,
      container.style.height = `${window.innerHeight - 100}px`;
      if (!PDFObject.default.embed(fileUrl, container)) {
        throw new Error('Unable to render the PDF file');
      }
    },
  };
  registerFileRenderPlugin(plugin);
}
