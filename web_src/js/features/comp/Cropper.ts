import {toggleElem} from '../../utils/dom.ts';

export async function initAvatarUploader(fileInput: HTMLInputElement) {
  const panel = fileInput.nextElementSibling as HTMLElement;
  if (!panel?.matches('.cropper-panel')) throw new Error('Missing cropper panel for avatar uploader');
  await import('@github/image-crop-element');
  const cropper = panel.querySelector('image-crop')!;
  const image = new Image();
  let initialWidth: number | null = null;
  let crop: HTMLElementEventMap['image-crop-change']['detail'] | null = null;
  let cropping = false;

  cropper.addEventListener('image-crop-change', ({detail}) => {
    initialWidth ??= detail.width;
    crop = detail;
  });

  fileInput.addEventListener('input', () => {
    crop = initialWidth = null;
    const file = fileInput.files![0];
    toggleElem(panel, Boolean(file));
    if (file) image.src = cropper.src = URL.createObjectURL(file);
  });

  const form = fileInput.form!;
  form.addEventListener('submit', (e) => {
    const {naturalWidth, naturalHeight} = image;
    if (!crop || (naturalWidth === naturalHeight && crop.width === initialWidth)) return; // a full selection of a square image uploads the original to keep animations
    e.preventDefault();
    if (cropping) return;
    cropping = true;
    const side = Math.min(crop.width, crop.height, naturalWidth, naturalHeight);
    const x = Math.max(0, Math.min(crop.x, naturalWidth - side));
    const y = Math.max(0, Math.min(crop.y, naturalHeight - side));
    const canvas = document.createElement('canvas');
    canvas.width = canvas.height = side;
    canvas.getContext('2d')!.drawImage(image, x, y, side, side, 0, 0, side, side);
    canvas.toBlob((blob) => {
      const file = fileInput.files![0];
      const dataTransfer = new DataTransfer();
      dataTransfer.items.add(new File([blob!], file.name.replace(/\.[^.]+$/, '.png'), {type: 'image/png', lastModified: file.lastModified}));
      fileInput.files = dataTransfer.files;
      crop = null;
      cropping = false;
      form.requestSubmit(e.submitter);
    });
  });
}
