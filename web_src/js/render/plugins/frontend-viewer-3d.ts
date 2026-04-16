import type {FrontendRenderFunc} from '../plugin.ts';
import {basename} from '../../utils.ts';
import * as OV from 'online-3d-viewer';
import {colord} from 'colord';

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

export const frontendRender: FrontendRenderFunc = async (opts): Promise<boolean> => {
  try {
    const bgColor = colord(getComputedStyle(document.body).backgroundColor).toRgb();
    const primaryColor = colord(getComputedStyle(document.documentElement).getPropertyValue('--color-primary').trim()).toRgb();
    const viewer = new OV.EmbeddedViewer(opts.container, {
      backgroundColor: new OV.RGBAColor(bgColor.r, bgColor.g, bgColor.b, 255),
      defaultColor: new OV.RGBColor(primaryColor.r, primaryColor.g, primaryColor.b),
      edgeSettings: new OV.EdgeSettings(false, new OV.RGBColor(0, 0, 0), 1),
    });
    const blob = new Blob([opts.contentBytes()]);
    const file = new File([blob], basename(opts.treePath));
    viewer.LoadModelFromFileList([file]);
    return true;
  } catch {
    return false;
  }
};
