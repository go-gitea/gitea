import type {FileRenderPlugin} from '../plugin.ts';
import {basename, extname} from '../../utils.ts';
import {getRealBackgroundColor} from '../../markup/render-iframe.ts';
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
    // iframe can't read same-origin URLs, and ships bytes+bgcolor over postMessage.
    async render(container: HTMLElement, fileUrl: string): Promise<void> {
      const viewerUrl = container.getAttribute('data-viewer3d-url')!;
      const bgcolor = getRealBackgroundColor(container);
      const primary = getComputedStyle(document.documentElement).getPropertyValue('--color-primary').trim();

      const iframe = document.createElement('iframe');
      iframe.setAttribute('sandbox', 'allow-scripts');
      iframe.className = 'tw-w-full tw-h-full tw-border-0';
      iframe.srcdoc = html`
        <!DOCTYPE html>
        <html><head><meta charset="utf-8"><style>html,body{margin:0;height:100%;overflow:hidden;background:${bgcolor}}#viewer{width:100%;height:100%}</style></head><body><div id="viewer"></div><script type="module" src="${viewerUrl}"></script></body></html>
      `;
      container.append(iframe);

      const [response] = await Promise.all([
        GET(fileUrl),
        new Promise((resolve) => iframe.addEventListener('load', resolve, {once: true})),
      ]);
      if (!response.ok) throw new Error(`failed to fetch file: ${response.status}`);
      const bytes = await response.arrayBuffer();
      iframe.contentWindow!.postMessage({filename: basename(fileUrl), bytes, bgcolor, primary}, '*', [bytes]);
    },
  };
}
