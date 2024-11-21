import Cropper from 'cropperjs';
import {showElem} from '../../utils/dom.ts';

export function initCompCropper() {
  const cropperContainer = document.querySelector('#cropper-panel');
  if (!cropperContainer) {
    return;
  }

  let filename;
  let cropper;
  const source = document.querySelector('#cropper-source');
  const result = document.querySelector('#cropper-result');
  const input = document.querySelector('#new-avatar');

  const done = function (url: string): void {
    source.src = url;
    result.src = url;

    if (cropper) {
      cropper.replace(url);
    } else {
      cropper = new Cropper(source, {
        aspectRatio: 1,
        viewMode: 1,
        autoCrop: false,
        crop() {
          const canvas = cropper.getCroppedCanvas();
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

  input.addEventListener('change', (e) => {
    const files = e.target.files;

    let reader;
    let file;
    if (files && files.length > 0) {
      file = files[0];
      filename = file.name;
      if (URL) {
        done(URL.createObjectURL(file));
      } else if (FileReader) {
        reader = new FileReader();
        reader.addEventListener('load', () => {
          done(reader.result);
        });
        reader.readAsDataURL(file);
      }
    }
  });
}
