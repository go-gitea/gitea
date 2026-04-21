import type {InplaceRenderPlugin} from '../plugin.ts';

export function newInplacePluginPdfViewer(): InplaceRenderPlugin {
  return {
    name: 'pdf-viewer',

    canHandle(filename: string, _mimeType: string): boolean {
      return filename.toLowerCase().endsWith('.pdf');
    },

    async render(container: HTMLElement, fileUrl: string): Promise<void> {
      const PDFObject = await import('pdfobject');
      // TODO: the PDFObject library does not support dynamic height adjustment,
      // TODO: it seems that this render must be an inplace render, because the URL must be accessible from the current context
      container.style.height = `${window.innerHeight - 100}px`;
      if (!PDFObject.default.embed(fileUrl, container)) {
        throw new Error('Unable to render the PDF file');
      }
    },
  };
}
