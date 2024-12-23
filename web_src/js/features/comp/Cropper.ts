import {showElem, type DOMEvent} from '../../utils/dom.ts';

type CropperOpts = {
  container: HTMLElement,
  imageSource: HTMLImageElement,
  fileInput: HTMLInputElement,
}

export async function initCompCropper({container, fileInput, imageSource}: CropperOpts) {
  const {default: Cropper} = await import(/* webpackChunkName: "cropperjs" */'cropperjs');
  let currentFileName = '';
  let currentFileLastModified = 0;
  const cropper = new Cropper(imageSource, {
    aspectRatio: 1,
    viewMode: 2,
    autoCrop: false,
    crop() {
      const canvas = cropper.getCroppedCanvas();
      canvas.toBlob((blob) => {
        const croppedFileName = currentFileName.replace(/\.[^.]{3,4}$/, '.png');
        const croppedFile = new File([blob], croppedFileName, {type: 'image/png', lastModified: currentFileLastModified});
        const dataTransfer = new DataTransfer();
        dataTransfer.items.add(croppedFile);
        fileInput.files = dataTransfer.files;
      });
    },
  });

  fileInput.addEventListener('input', (e: DOMEvent<Event, HTMLInputElement>) => {
    const files = e.target.files;
    if (files?.length > 0) {
      currentFileName = files[0].name;
      currentFileLastModified = files[0].lastModified;
      const fileURL = URL.createObjectURL(files[0]);
      imageSource.src = fileURL;
      cropper.replace(fileURL);
      showElem(container);
    }
  });
}
