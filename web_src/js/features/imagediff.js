import $ from 'jquery';
import {GET} from '../modules/fetch.js';
import {hideElem, loadElem, queryElemChildren} from '../utils/dom.js';
import {parseDom} from '../utils.js';

function getDefaultSvgBoundsIfUndefined(text, src) {
  const DefaultSize = 300;
  const MaxSize = 99999;

  const svgDoc = parseDom(text, 'image/svg+xml');
  const svg = svgDoc.documentElement;
  const width = svg?.width?.baseVal;
  const height = svg?.height?.baseVal;
  if (width === undefined || height === undefined) {
    return null; // in case some svg is invalid or doesn't have the width/height
  }
  if (width.unitType === SVGLength.SVG_LENGTHTYPE_PERCENTAGE || height.unitType === SVGLength.SVG_LENGTHTYPE_PERCENTAGE) {
    const img = new Image();
    img.src = src;
    if (img.width > 1 && img.width < MaxSize && img.height > 1 && img.height < MaxSize) {
      return {
        width: img.width,
        height: img.height,
      };
    }
    if (svg.hasAttribute('viewBox')) {
      const viewBox = svg.viewBox.baseVal;
      return {
        width: DefaultSize,
        height: DefaultSize * viewBox.width / viewBox.height,
      };
    }
    return {
      width: DefaultSize,
      height: DefaultSize,
    };
  }
  return null;
}

function createContext(imageAfter, imageBefore) {
  const sizeAfter = {
    width: imageAfter?.width || 0,
    height: imageAfter?.height || 0,
  };
  const sizeBefore = {
    width: imageBefore?.width || 0,
    height: imageBefore?.height || 0,
  };
  const maxSize = {
    width: Math.max(sizeBefore.width, sizeAfter.width),
    height: Math.max(sizeBefore.height, sizeAfter.height),
  };

  return {
    imageAfter,
    imageBefore,
    sizeAfter,
    sizeBefore,
    maxSize,
    ratio: [
      Math.floor(maxSize.width - sizeAfter.width) / 2,
      Math.floor(maxSize.height - sizeAfter.height) / 2,
      Math.floor(maxSize.width - sizeBefore.width) / 2,
      Math.floor(maxSize.height - sizeBefore.height) / 2,
    ],
  };
}

export function initImageDiff() {
  $('.image-diff:not([data-image-diff-loaded])').each(async function() {
    const $container = $(this);
    this.setAttribute('data-image-diff-loaded', 'true');

    // the container may be hidden by "viewed" checkbox, so use the parent's width for reference
    const diffContainerWidth = Math.max($container.closest('.diff-file-box').width() - 300, 100);

    const imageInfos = [{
      path: this.getAttribute('data-path-after'),
      mime: this.getAttribute('data-mime-after'),
      $images: $container.find('img.image-after'), // matches 3 <img>
      boundsInfo: this.querySelector('.bounds-info-after'),
    }, {
      path: this.getAttribute('data-path-before'),
      mime: this.getAttribute('data-mime-before'),
      $images: $container.find('img.image-before'), // matches 3 <img>
      boundsInfo: this.querySelector('.bounds-info-before'),
    }];

    await Promise.all(imageInfos.map(async (info) => {
      const [success] = await Promise.all(Array.from(info.$images, (img) => {
        return loadElem(img, info.path);
      }));
      // only the first images is associated with boundsInfo
      if (!success && info.boundsInfo) info.boundsInfo.textContent = '(image error)';
      if (info.mime === 'image/svg+xml') {
        const resp = await GET(info.path);
        const text = await resp.text();
        const bounds = getDefaultSvgBoundsIfUndefined(text, info.path);
        if (bounds) {
          info.$images.each(function() {
            this.setAttribute('width', bounds.width);
            this.setAttribute('height', bounds.height);
          });
          hideElem(info.boundsInfo);
        }
      }
    }));

    const $imagesAfter = imageInfos[0].$images;
    const $imagesBefore = imageInfos[1].$images;

    initSideBySide(this, createContext($imagesAfter[0], $imagesBefore[0]));
    if ($imagesAfter.length > 0 && $imagesBefore.length > 0) {
      initSwipe(createContext($imagesAfter[1], $imagesBefore[1]));
      initOverlay(createContext($imagesAfter[2], $imagesBefore[2]));
    }

    queryElemChildren(this, '.image-diff-tabs', (el) => el.classList.remove('is-loading'));

    function initSideBySide(container, sizes) {
      let factor = 1;
      if (sizes.maxSize.width > (diffContainerWidth - 24) / 2) {
        factor = (diffContainerWidth - 24) / 2 / sizes.maxSize.width;
      }

      const widthChanged = sizes.imageAfter && sizes.imageBefore && sizes.imageAfter.naturalWidth !== sizes.imageBefore.naturalWidth;
      const heightChanged = sizes.imageAfter && sizes.imageBefore && sizes.imageAfter.naturalHeight !== sizes.imageBefore.naturalHeight;
      if (sizes.imageAfter) {
        const boundsInfoAfterWidth = container.querySelector('.bounds-info-after .bounds-info-width');
        if (boundsInfoAfterWidth) {
          boundsInfoAfterWidth.textContent = `${sizes.imageAfter.naturalWidth}px`;
          boundsInfoAfterWidth.classList.toggle('green', widthChanged);
        }
        const boundsInfoAfterHeight = container.querySelector('.bounds-info-after .bounds-info-height');
        if (boundsInfoAfterHeight) {
          boundsInfoAfterHeight.textContent = `${sizes.imageAfter.naturalHeight}px`;
          boundsInfoAfterHeight.classList.toggle('green', heightChanged);
        }
      }

      if (sizes.imageBefore) {
        const boundsInfoBeforeWidth = container.querySelector('.bounds-info-before .bounds-info-width');
        if (boundsInfoBeforeWidth) {
          boundsInfoBeforeWidth.textContent = `${sizes.imageBefore.naturalWidth}px`;
          boundsInfoBeforeWidth.classList.toggle('red', widthChanged);
        }
        const boundsInfoBeforeHeight = container.querySelector('.bounds-info-before .bounds-info-height');
        if (boundsInfoBeforeHeight) {
          boundsInfoBeforeHeight.textContent = `${sizes.imageBefore.naturalHeight}px`;
          boundsInfoBeforeHeight.classList.add('red', heightChanged);
        }
      }

      if (sizes.imageAfter) {
        const container = sizes.imageAfter.parentNode;
        sizes.imageAfter.style.width = `${sizes.sizeAfter.width * factor}px`;
        sizes.imageAfter.style.height = `${sizes.sizeAfter.height * factor}px`;
        container.style.margin = '10px auto';
        container.style.width = `${sizes.sizeAfter.width * factor + 2}px`;
        container.style.height = `${sizes.sizeAfter.height * factor + 2}px`;
      }

      if (sizes.imageBefore) {
        const container = sizes.imageBefore.parentNode;
        sizes.imageBefore.style.width = `${sizes.sizeBefore.width * factor}px`;
        sizes.imageBefore.style.height = `${sizes.sizeBefore.height * factor}px`;
        container.style.margin = '10px auto';
        container.style.width = `${sizes.sizeBefore.width * factor + 2}px`;
        container.style.height = `${sizes.sizeBefore.height * factor + 2}px`;
      }
    }

    function initSwipe(sizes) {
      let factor = 1;
      if (sizes.maxSize.width > diffContainerWidth - 12) {
        factor = (diffContainerWidth - 12) / sizes.maxSize.width;
      }

      if (sizes.imageAfter) {
        const container = sizes.imageAfter.parentNode;
        const swipeFrame = container.parentNode;
        sizes.imageAfter.style.width = `${sizes.sizeAfter.width * factor}px`;
        sizes.imageAfter.style.height = `${sizes.sizeAfter.height * factor}px`;
        container.style.margin = `0px ${sizes.ratio[0] * factor}px`;
        container.style.width = `${sizes.sizeAfter.width * factor + 2}px`;
        container.style.height = `${sizes.sizeAfter.height * factor + 2}px`;
        swipeFrame.style.padding = `${sizes.ratio[1] * factor}px 0 0 0`;
        swipeFrame.style.width = `${sizes.maxSize.width * factor + 2}px`;
      }

      if (sizes.imageBefore) {
        const container = sizes.imageBefore.parentNode;
        const swipeFrame = container.parentNode;
        sizes.imageBefore.style.width = `${sizes.sizeBefore.width * factor}px`;
        sizes.imageBefore.style.height = `${sizes.sizeBefore.height * factor}px`;
        container.style.margin = `${sizes.ratio[3] * factor}px ${sizes.ratio[2] * factor}px`;
        container.style.width = `${sizes.sizeBefore.width * factor + 2}px`;
        container.style.height = `${sizes.sizeBefore.height * factor + 2}px`;
        swipeFrame.style.width = `${sizes.maxSize.width * factor + 2}px`;
        swipeFrame.style.height = `${sizes.maxSize.height * factor + 2}px`;
      }

      // extra height for inner "position: absolute" elements
      const swipe = $container.find('.diff-swipe')[0];
      if (swipe) {
        swipe.style.width = `${sizes.maxSize.width * factor + 2}px`;
        swipe.style.height = `${sizes.maxSize.height * factor + 30}px`;
      }

      $container.find('.swipe-bar').on('mousedown', function(e) {
        e.preventDefault();

        const $swipeBar = $(this);
        const $swipeFrame = $swipeBar.parent();
        const width = $swipeFrame.width() - $swipeBar.width() - 2;

        $(document).on('mousemove.diff-swipe', (e2) => {
          e2.preventDefault();

          const value = Math.max(0, Math.min(e2.clientX - $swipeFrame.offset().left, width));
          $swipeBar[0].style.left = `${value}px`;
          $container.find('.swipe-container')[0].style.width = `${$swipeFrame.width() - value}px`;

          $(document).on('mouseup.diff-swipe', () => {
            $(document).off('.diff-swipe');
          });
        });
      });
    }

    function initOverlay(sizes) {
      let factor = 1;
      if (sizes.maxSize.width > diffContainerWidth - 12) {
        factor = (diffContainerWidth - 12) / sizes.maxSize.width;
      }

      if (sizes.imageAfter) {
        const container = sizes.imageAfter.parentNode;
        sizes.imageAfter.style.width = `${sizes.sizeAfter.width * factor}px`;
        sizes.imageAfter.style.height = `${sizes.sizeAfter.height * factor}px`;
        container.style.margin = `${sizes.ratio[1] * factor}px ${sizes.ratio[0] * factor}px`;
        container.style.width = `${sizes.sizeAfter.width * factor + 2}px`;
        container.style.height = `${sizes.sizeAfter.height * factor + 2}px`;
      }

      if (sizes.imageBefore) {
        const container = sizes.imageBefore.parentNode;
        const overlayFrame = container.parentNode;
        sizes.imageBefore.style.width = `${sizes.sizeBefore.width * factor}px`;
        sizes.imageBefore.style.height = `${sizes.sizeBefore.height * factor}px`;
        container.style.margin = `${sizes.ratio[3] * factor}px ${sizes.ratio[2] * factor}px`;
        container.style.width = `${sizes.sizeBefore.width * factor + 2}px`;
        container.style.height = `${sizes.sizeBefore.height * factor + 2}px`;

        // some inner elements are `position: absolute`, so the container's height must be large enough
        overlayFrame.style.width = `${sizes.maxSize.width * factor + 2}px`;
        overlayFrame.style.height = `${sizes.maxSize.height * factor + 2}px`;
      }

      const rangeInput = $container[0].querySelector('input[type="range"]');
      function updateOpacity() {
        if (sizes.imageAfter) {
          sizes.imageAfter.parentNode.style.opacity = `${rangeInput.value / 100}`;
        }
      }
      rangeInput?.addEventListener('input', updateOpacity);
      updateOpacity();
    }
  });
}
