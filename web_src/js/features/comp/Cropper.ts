import {toggleElem} from '../../utils/dom.ts';

type CropArea = {x: number, y: number, width: number};

export async function initAvatarUploader(fileInput: HTMLInputElement) {
  const panel = fileInput.nextElementSibling as HTMLElement;
  if (!panel?.matches('.cropper-panel')) throw new Error('Missing cropper panel for avatar uploader');
  await import('@github/image-crop-element');
  const cropper = panel.querySelector('image-crop')!;
  const image = new Image();
  let initialCrop = '';
  let crop: CropArea | null = null;
  let cropping = false;

  cropper.addEventListener('image-crop-change', ({detail}) => {
    const key = JSON.stringify(detail);
    initialCrop ||= key;
    crop = key === initialCrop || !detail.width ? null : detail; // untouched selection uploads the original to keep animated images
  });

  fileInput.addEventListener('input', () => {
    crop = null;
    initialCrop = '';
    const file = fileInput.files![0];
    toggleElem(panel, Boolean(file));
    if (file) image.src = cropper.src = URL.createObjectURL(file);
  });

  const form = fileInput.form!;
  form.addEventListener('submit', (e) => {
    if (!crop) return;
    e.preventDefault();
    if (cropping) return;
    cropping = true;
    const {x, y, width} = crop;
    const side = Math.min(width, image.naturalWidth - x, image.naturalHeight - y); // the element rounds x and width separately
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
