import type {FileRenderPlugin} from '../plugin.ts';
import {isDarkTheme} from '../../utils.ts';
import {request} from '../../modules/fetch.ts';

export function newRenderPluginExcalidrawViewer(): FileRenderPlugin {
  return {
    name: 'excalidraw-viewer',

    canHandle(filename: string, _mimeType: string): boolean {
      return filename.toLowerCase().endsWith('.excalidraw');
    },

    async render(container: HTMLElement, fileUrl: string): Promise<void> {
      const {exportToSvg} = await import(/* webpackChunkName: "excalidraw/utils" */ '@excalidraw/utils');
      const data = await request(fileUrl);
      const excalidrawJson = await data.json();
      const svg = await exportToSvg({
        elements: excalidrawJson.elements,
        appState: {
          ...excalidrawJson.appState,
          exportWithDarkMode: isDarkTheme(),
        },
        files: excalidrawJson.files,
        skipInliningFonts: true,
      });
      container.style.display = 'flex';
      container.style.justifyContent = 'center';
      svg.style.maxWidth = '80%';
      svg.style.height = 'auto';
      container.append(svg);
    },
  };
}
