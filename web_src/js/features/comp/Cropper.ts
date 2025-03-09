import {CropperCanvas, CropperImage} from 'cropperjs';
import {createElementFromHTML, showElem, type DOMEvent} from '../../utils/dom.ts';

type CropperOpts = {
  container: HTMLElement,
  imageSource: HTMLImageElement,
  fileInput: HTMLInputElement,
}

async function initCompCropper({container, fileInput, imageSource}: CropperOpts) {
  await import(/* webpackChunkName: "cropperjs" */'cropperjs');
  let currentFileName = '';
  let currentFileLastModified = 0;

  const canvasEl = createElementFromHTML<CropperCanvas>(`
    <cropper-canvas background>
      <cropper-image src="${imageSource.src}" rotatable scalable skewable translatable></cropper-image>
      <cropper-shade hidden></cropper-shade>
      <cropper-handle action="select" plain></cropper-handle>
      <cropper-selection initial-coverage="0.5" initial-aspect-ratio="1" movable resizable>
        <cropper-grid role="grid" covered></cropper-grid>
        <cropper-crosshair centered></cropper-crosshair>
        <cropper-handle action="move" theme-color="#ffffff23"></cropper-handle>
        <cropper-handle action="n-resize"></cropper-handle>
        <cropper-handle action="e-resize"></cropper-handle>
        <cropper-handle action="s-resize"></cropper-handle>
        <cropper-handle action="w-resize"></cropper-handle>
        <cropper-handle action="ne-resize"></cropper-handle>
        <cropper-handle action="nw-resize"></cropper-handle>
        <cropper-handle action="se-resize"></cropper-handle>
        <cropper-handle action="sw-resize"></cropper-handle>
      </cropper-selection>
    </cropper-canvas>
  `);

  const imgEl = canvasEl.querySelector<CropperImage>('cropper-image');

  canvasEl.addEventListener('action', async (e) => {
    const canvas = await (e.target as CropperCanvas).$toCanvas();
    canvas.toBlob((blob) => {
      const croppedFileName = currentFileName.replace(/\.[^.]{3,4}$/, '.png');
      const croppedFile = new File([blob], croppedFileName, {type: 'image/png', lastModified: currentFileLastModified});
      const dataTransfer = new DataTransfer();
      dataTransfer.items.add(croppedFile);
      fileInput.files = dataTransfer.files;
    });
  });

  imageSource.replaceWith(canvasEl);

  fileInput.addEventListener('input', (e: DOMEvent<Event, HTMLInputElement>) => {
    const files = e.target.files;
    if (files?.length > 0) {
      currentFileName = files[0].name;
      currentFileLastModified = files[0].lastModified;
      const fileURL = URL.createObjectURL(files[0]);
      imageSource.src = fileURL;
      imgEl.src = fileURL;
      showElem(container);
    }
  });
}

export async function initAvatarUploaderWithCropper(fileInput: HTMLInputElement) {
  const panel = fileInput.nextElementSibling as HTMLElement;
  if (!panel?.matches('.cropper-panel')) throw new Error('Missing cropper panel for avatar uploader');
  const imageSource = panel.querySelector<HTMLImageElement>('.cropper-source');
  await initCompCropper({container: panel, fileInput, imageSource});
}
