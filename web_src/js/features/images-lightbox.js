import SimpleLightbox from 'simple-lightbox';

export function initImagesLightbox() {
  new SimpleLightbox({
    elements: '.js-lightbox'
  });
}
