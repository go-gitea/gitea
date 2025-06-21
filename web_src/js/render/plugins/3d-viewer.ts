import type {FileRenderPlugin} from '../../modules/file-render-plugin.ts';
import {registerFileRenderPlugin} from '../../modules/file-render-plugin.ts';

/**
 * 3D model file render plugin
 *
 * support common 3D model file formats, use online-3d-viewer library for rendering
 */
export function register3DViewerPlugin(): void {
  // supported 3D file extensions
  const SUPPORTED_EXTENSIONS = [
    '.3dm', '.3ds', '.3mf', '.amf', '.bim', '.brep',
    '.dae', '.fbx', '.fcstd', '.glb', '.gltf',
    '.ifc', '.igs', '.iges', '.stp', '.step',
    '.stl', '.obj', '.off', '.ply', '.wrl',
  ];

  // create and register plugin
  const plugin: FileRenderPlugin = {
    name: '3d-model-viewer',

    // check if file extension is supported 3D file
    canHandle(filename: string, _mimeType: string): boolean {
      const ext = filename.substring(filename.lastIndexOf('.')).toLowerCase();
      const canHandle = SUPPORTED_EXTENSIONS.includes(ext);
      return canHandle;
    },

    // render 3D model
    async render(container: HTMLElement, fileUrl: string): Promise<void> {
      // add loading indicator
      container.classList.add('is-loading');

      try {
        // dynamically load 3D rendering library
        const OV = await import(/* webpackChunkName: "online-3d-viewer" */'online-3d-viewer');

        // configure container style
        container.classList.add('model3d-content');

        // initialize 3D viewer
        const viewer = new OV.EmbeddedViewer(container, {
          backgroundColor: new OV.RGBAColor(59, 68, 76, 0), // transparent
          defaultColor: new OV.RGBColor(65, 131, 196),
          edgeSettings: new OV.EdgeSettings(false, new OV.RGBColor(0, 0, 0), 1),
        });

        // load model from url
        viewer.LoadModelFromUrlList([fileUrl]);
      } catch (error) {
        // handle render error
        console.error('error rendering 3D model:', error);

        // add error message and download button
        const fallbackText = container.getAttribute('data-fallback-text') || 'View Raw File';
        container.innerHTML = `
          <div class="ui error message">
            <div class="header">Failed to render 3D model</div>
            <p>The 3D model could not be displayed in the browser.</p>
            <a class="ui basic button" href="${fileUrl}" target="_blank">${fallbackText}</a>
          </div>
        `;
      } finally {
        // remove loading state
        container.classList.remove('is-loading');
      }
    },
  };

  // register plugin
  registerFileRenderPlugin(plugin);
}
