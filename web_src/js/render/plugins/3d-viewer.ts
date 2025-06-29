import type {FileRenderPlugin} from '../plugin.ts';
import {extname} from '../../utils.ts';

// support common 3D model file formats, use online-3d-viewer library for rendering
export function newRenderPlugin3DViewer(): FileRenderPlugin {
  // Some extensions are text-based formats:
  // .3mf .amf .brep: XML
  // .fbx: XML or BINARY
  // .dae .gltf: JSON
  // .ifc, .igs, .iges, .stp, .step are: TEXT
  // .stl .ply: TEXT or BINARY
  // .obj .off .wrl: TEXT
  // TODO: So we need to be able to render when the file is recognized as plaintext file by backend
  const SUPPORTED_EXTENSIONS = [
    '.3dm', '.3ds', '.3mf', '.amf', '.bim', '.brep',
    '.dae', '.fbx', '.fcstd', '.glb', '.gltf',
    '.ifc', '.igs', '.iges', '.stp', '.step',
    '.stl', '.obj', '.off', '.ply', '.wrl',
  ];

  return {
    name: '3d-model-viewer',

    canHandle(filename: string, _mimeType: string): boolean {
      const ext = extname(filename).toLowerCase();
      return SUPPORTED_EXTENSIONS.includes(ext);
    },

    async render(container: HTMLElement, fileUrl: string): Promise<void> {
      const OV = await import(/* webpackChunkName: "online-3d-viewer" */'online-3d-viewer');
      container.classList.add('model3d-content');
      const viewer = new OV.EmbeddedViewer(container, {
        backgroundColor: new OV.RGBAColor(59, 68, 76, 0),
        defaultColor: new OV.RGBColor(65, 131, 196),
        edgeSettings: new OV.EdgeSettings(false, new OV.RGBColor(0, 0, 0), 1),
      });
      viewer.LoadModelFromUrlList([fileUrl]);
    },
  };
}
