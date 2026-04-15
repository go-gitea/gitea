import * as OV from 'online-3d-viewer';
import {colord} from 'colord';

window.addEventListener('message', (event: MessageEvent) => {
  if (event.source !== window.parent) return;
  if (event.data?.giteaIframeCmd !== 'render') return;
  const {filename, bytes, primary} = event.data;
  // Background is applied by external-render-helper before this message arrives.
  const bgColor = colord(getComputedStyle(document.body).backgroundColor).toRgb();
  const primaryColor = colord(primary).toRgb();
  const viewer = new OV.EmbeddedViewer(document.querySelector<HTMLElement>('#viewer')!, {
    backgroundColor: new OV.RGBAColor(bgColor.r, bgColor.g, bgColor.b, 255),
    defaultColor: new OV.RGBColor(primaryColor.r, primaryColor.g, primaryColor.b),
    edgeSettings: new OV.EdgeSettings(false, new OV.RGBColor(0, 0, 0), 1),
  });
  viewer.LoadModelFromFileList([new File([bytes], filename)]);
});
