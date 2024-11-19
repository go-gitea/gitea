import Cropper from 'cropperjs';

export function initCompCropper() {
  if (!document.querySelector('#cropper')) {
    return;
  }

  let filename;
  const image = document.querySelector('#image');
  const result = document.querySelector('#result');
  const input = document.querySelector('#new-avatar');

  const done = function (url) {
    image.src = url;

    const cropper = new Cropper(image, {
      aspectRatio: 1,
      viewMode: 1,
      crop() {
        const canvas = cropper.getCroppedCanvas();
        result.src = canvas.toDataURL();
        canvas.toBlob((blob) => {
          const file = new File([blob], filename, {type: 'image/jpeg', lastModified: Date.now()});
          const container = new DataTransfer();
          container.items.add(file);
          input.files = container.files;
        });
      },
    });
    document.querySelector('#cropper').classList.remove('hidden');
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
