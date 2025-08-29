import type {FileRenderPlugin} from '../plugin.ts';

export function newRenderPluginPdfViewer(): FileRenderPlugin {
  return {
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
}
