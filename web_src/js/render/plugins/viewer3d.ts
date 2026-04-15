import type {FileRenderPlugin} from '../plugin.ts';
import {basename, extname} from '../../utils.ts';
import {initMarkupRenderIframe} from '../../markup/render-iframe.ts';
import {GET} from '../../modules/fetch.ts';
import {html} from '../../utils/html.ts';

const SUPPORTED_EXTENSIONS = [
  '.3dm', '.3ds', '.3mf', '.amf', '.bim', '.brep',
  '.dae', '.fbx', '.fcstd', '.glb', '.gltf',
  '.ifc', '.igs', '.iges', '.stp', '.step',
  '.stl', '.obj', '.off', '.ply', '.wrl',
];

export function newRenderPlugin3DViewer(): FileRenderPlugin {
  return {
    name: '3d-model-viewer',

    canHandle(filename: string, _mimeType: string): boolean {
      return SUPPORTED_EXTENSIONS.includes(extname(filename).toLowerCase());
    },

    // Render inside a sandboxed iframe (`allow-scripts` without `allow-same-origin` → null origin)
    // so the 3D library runs isolated from the parent. The parent fetches the file because the
    // null-origin iframe can't read same-origin URLs, and ships bytes over postMessage.
    async render(container: HTMLElement, fileUrl: string): Promise<void> {
      const viewerUrl = container.getAttribute('data-viewer3d-url')!;
      const helperUrl = container.getAttribute('data-external-render-helper-url')!;
      const primary = getComputedStyle(document.documentElement).getPropertyValue('--color-primary').trim();

      const iframe = document.createElement('iframe');
      iframe.setAttribute('sandbox', 'allow-scripts');
      iframe.className = 'external-render-iframe tw-w-full tw-h-full tw-border-0';
      iframe.srcdoc = html`
        <!DOCTYPE html>
        <html><head><meta charset="utf-8"><style>html,body{margin:0;height:100%;overflow:hidden}#viewer{width:100%;height:100%}</style><script crossorigin src="${helperUrl}"></script></head><body><div id="viewer"></div><script type="module" src="${viewerUrl}"></script></body></html>
      `;
      container.append(iframe);

      // Hook into the existing iframe framework: assigns iframe.id, listens for resize/open-link,
      // posts the init message that the helper script inside the iframe consumes.
      initMarkupRenderIframe(container);

      const [response] = await Promise.all([
        GET(fileUrl),
        new Promise((resolve) => iframe.addEventListener('load', resolve, {once: true})),
      ]);
      if (!response.ok) throw new Error(`failed to fetch file: ${response.status}`);
      const bytes = await response.arrayBuffer();
      iframe.contentWindow!.postMessage({
        giteaIframeCmd: 'render',
        giteaIframeId: iframe.id,
        filename: basename(fileUrl),
        bytes,
        primary,
      }, '*', [bytes]);
    },
  };
}
