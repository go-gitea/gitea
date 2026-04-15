import * as OV from 'online-3d-viewer';
import {colord} from 'colord';

const modelDataScript = document.querySelector<HTMLElement>('#modelData')!;
const fileName = modelDataScript.getAttribute('data-filename')!;
const binaryString = atob(modelDataScript.textContent);
modelDataScript.remove(); // free the embedded base64 string for GC before decoding
const fileBytes = new Uint8Array(binaryString.length);
for (let idx = 0; idx < binaryString.length; idx++) {
  fileBytes[idx] = binaryString.charCodeAt(idx);
}

const bgColor = colord(getComputedStyle(document.body).backgroundColor).toRgb();
const primaryColor = colord(getComputedStyle(document.documentElement).getPropertyValue('--color-primary').trim()).toRgb();

const viewer = new OV.EmbeddedViewer(document.querySelector<HTMLElement>('#viewer')!, {
  backgroundColor: new OV.RGBAColor(bgColor.r, bgColor.g, bgColor.b, 255),
  defaultColor: new OV.RGBColor(primaryColor.r, primaryColor.g, primaryColor.b),
  edgeSettings: new OV.EdgeSettings(false, new OV.RGBColor(0, 0, 0), 1),
});
viewer.LoadModelFromFileList([new File([fileBytes], fileName)]);
