import * as OV from 'online-3d-viewer';

const modelDataScript = document.querySelector<HTMLElement>('#modelData')!;
const fileName = modelDataScript.getAttribute('data-filename')!;
const fileBytes = Uint8Array.from(atob(modelDataScript.textContent), (c) => c.charCodeAt(0));

const container = document.querySelector<HTMLElement>('#viewer')!;
const viewer = new OV.EmbeddedViewer(container, {
  backgroundColor: new OV.RGBAColor(59, 68, 76, 0),
  defaultColor: new OV.RGBColor(65, 131, 196),
  edgeSettings: new OV.EdgeSettings(false, new OV.RGBColor(0, 0, 0), 1),
});
viewer.LoadModelFromFileList([new File([fileBytes], fileName)]);
