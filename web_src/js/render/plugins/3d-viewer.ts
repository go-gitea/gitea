import type {FileRenderPlugin} from '../plugin.ts';
import {extname} from '../../utils.ts';

// support common 3D model file formats, use online-3d-viewer library for rendering

/* a simple text STL file example:
solid SimpleTriangle
  facet normal 0 0 1
    outer loop
      vertex 0 0 0
      vertex 1 0 0
      vertex 0 1 0
    endloop
  endfacet
endsolid SimpleTriangle
*/

export function newRenderPlugin3DViewer(): FileRenderPlugin {
  // Some extensions are text-based formats:
  // .3mf .amf .brep: XML
  // .fbx: XML or BINARY
  // .dae .gltf: JSON
  // .ifc, .igs, .iges, .stp, .step are: TEXT
  // .stl .ply: TEXT or BINARY
  // .obj .off .wrl: TEXT
  // So we need to be able to render when the file is recognized as plaintext file by backend.
  //
  // It needs more logic to make it overall right (render a text 3D model automatically):
  // we need to distinguish the ambiguous filename extensions.
  // For example: "*.obj, *.off, *.step" might be or not be a 3D model file.
  // So when it is a text file, we can't assume that "we only render it by 3D plugin",
  // otherwise the end users would be impossible to view its real content when the file is not a 3D model.
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
      // TODO: height and/or max-height?
      const OV = await import(/* webpackChunkName: "online-3d-viewer" */'online-3d-viewer');
      const viewer = new OV.EmbeddedViewer(container, {
        backgroundColor: new OV.RGBAColor(59, 68, 76, 0),
        defaultColor: new OV.RGBColor(65, 131, 196),
        edgeSettings: new OV.EdgeSettings(false, new OV.RGBColor(0, 0, 0), 1),
      });
      viewer.LoadModelFromUrlList([fileUrl]);
    },
  };
}
