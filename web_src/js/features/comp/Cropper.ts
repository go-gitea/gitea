import {showElem} from '../../utils/dom.ts';

export async function initCompCropper() {
  const cropperContainer = document.querySelector('#cropper-panel');
  if (!cropperContainer) {
    return;
  }

  const {default: Cropper} = await import(/* webpackChunkName: "cropperjs" */'cropperjs');

  const source = document.querySelector('#cropper-source');
  const result = document.querySelector('#cropper-result');
  const input = document.querySelector('#new-avatar');

  const done = function (url: string, filename: string): void {
    source.src = url;
    result.src = url;

    if (input._cropper) {
      input._cropper.replace(url);
    } else {
      input._cropper = new Cropper(source, {
        aspectRatio: 1,
        viewMode: 1,
        autoCrop: false,
        crop() {
          const canvas = input._cropper.getCroppedCanvas();
          result.src = canvas.toDataURL();
          canvas.toBlob((blob) => {
            const file = new File([blob], filename, {type: 'image/png', lastModified: Date.now()});
            const container = new DataTransfer();
            container.items.add(file);
            input.files = container.files;
          });
        },
      });
    }
    showElem(cropperContainer);
  };

  input.addEventListener('change', (e: Event & {target: HTMLInputElement}) => {
    const files = e.target.files;

    if (files?.length > 0) {
      done(URL.createObjectURL(files[0]), files[0].name);
    }
  });
}
