import {createElementFromHTML, hideElem, showElem, type DOMEvent} from '../../utils/dom.ts';
import {debounce} from 'perfect-debounce';
import type {CropperCanvas, CropperSelection} from 'cropperjs';

type CropperOpts = {
  container: HTMLElement,
  wrapper: HTMLDivElement,
  fileInput: HTMLInputElement,
}

async function initCompCropper({container, fileInput, wrapper}: CropperOpts) {
  await import(/* webpackChunkName: "cropperjs" */'cropperjs');

  fileInput.addEventListener('input', (e: DOMEvent<Event, HTMLInputElement>) => {
    if (!e.target.files?.length) {
      wrapper.replaceChildren();
      hideElem(container);
      return;
    }

    const [file] = e.target.files;
    const objectUrl = URL.createObjectURL(file);
    const canvasEl = createElementFromHTML<CropperCanvas>(`
      <cropper-canvas theme-color="var(--color-primary)">
        <cropper-image src="${objectUrl}" scalable skewable translatable></cropper-image>
        <cropper-shade hidden></cropper-shade>
        <cropper-handle action="select" plain></cropper-handle>
        <cropper-selection aspect-ratio="1" movable resizable>
          <cropper-handle action="move" theme-color="transparent"></cropper-handle>
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
    canvasEl.querySelector<CropperSelection>('cropper-selection').addEventListener('change', debounce(async (e) => {
      const selection = e.target as CropperSelection;
      if (!selection.width || !selection.height) return;
      const canvas = await selection.$toCanvas();

      canvas.toBlob((blob) => {
        const dataTransfer = new DataTransfer();
        dataTransfer.items.add(new File(
          [blob],
          file.name.replace(/\.[^.]{3,4}$/, '.png'),
          {type: 'image/png', lastModified: file.lastModified},
        ));
        fileInput.files = dataTransfer.files;
      });
    }, 200));

    wrapper.replaceChildren(canvasEl);
    showElem(container);
  });
}

export async function initAvatarUploaderWithCropper(fileInput: HTMLInputElement) {
  const panel = fileInput.nextElementSibling as HTMLElement;
  if (!panel?.matches('.cropper-panel')) throw new Error('Missing cropper panel for avatar uploader');
  const wrapper = panel.querySelector<HTMLImageElement>('.cropper-wrapper');
  await initCompCropper({container: panel, fileInput, wrapper});
}
