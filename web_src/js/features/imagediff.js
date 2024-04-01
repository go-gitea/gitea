import $ from 'jquery';
import {GET} from '../modules/fetch.js';
import {hideElem, loadElem} from '../utils/dom.js';
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

export function initImageDiff() {
  function createContext(image1, image2) {
    const size1 = {
      width: image1 && image1.width || 0,
      height: image1 && image1.height || 0,
    };
    const size2 = {
      width: image2 && image2.width || 0,
      height: image2 && image2.height || 0,
    };
    const max = {
      width: Math.max(size2.width, size1.width),
      height: Math.max(size2.height, size1.height),
    };

    return {
      $image1: $(image1),
      $image2: $(image2),
      size1,
      size2,
      max,
      ratio: [
        Math.floor(max.width - size1.width) / 2,
        Math.floor(max.height - size1.height) / 2,
        Math.floor(max.width - size2.width) / 2,
        Math.floor(max.height - size2.height) / 2,
      ],
    };
  }

  $('.image-diff:not([data-image-diff-loaded])').each(async function() {
    const $container = $(this);
    this.setAttribute('data-image-diff-loaded', 'true');

    // the container may be hidden by "viewed" checkbox, so use the parent's width for reference
    const diffContainerWidth = Math.max($container.closest('.diff-file-box').width() - 300, 100);

    const imageInfos = [{
      path: this.getAttribute('data-path-after'),
      mime: this.getAttribute('data-mime-after'),
      $images: $container.find('img.image-after'), // matches 3 <img>
      $boundsInfo: $container.find('.bounds-info-after'),
    }, {
      path: this.getAttribute('data-path-before'),
      mime: this.getAttribute('data-mime-before'),
      $images: $container.find('img.image-before'), // matches 3 <img>
      $boundsInfo: $container.find('.bounds-info-before'),
    }];

    await Promise.all(imageInfos.map(async (info) => {
      const [success] = await Promise.all(Array.from(info.$images, (img) => {
        return loadElem(img, info.path);
      }));
      // only the first images is associated with $boundsInfo
      if (!success) info.$boundsInfo.text('(image error)');
      if (info.mime === 'image/svg+xml') {
        const resp = await GET(info.path);
        const text = await resp.text();
        const bounds = getDefaultSvgBoundsIfUndefined(text, info.path);
        if (bounds) {
          info.$images.each(function() {
            this.setAttribute('width', bounds.width);
            this.setAttribute('height', bounds.height);
          });
          hideElem(info.$boundsInfo);
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

    this.querySelector(':scope > .image-diff-tabs')?.classList.remove('is-loading');

    function initSideBySide(container, sizes) {
      let factor = 1;
      if (sizes.max.width > (diffContainerWidth - 24) / 2) {
        factor = (diffContainerWidth - 24) / 2 / sizes.max.width;
      }

      const widthChanged = sizes.$image1.length !== 0 && sizes.$image2.length !== 0 && sizes.$image1[0].naturalWidth !== sizes.$image2[0].naturalWidth;
      const heightChanged = sizes.$image1.length !== 0 && sizes.$image2.length !== 0 && sizes.$image1[0].naturalHeight !== sizes.$image2[0].naturalHeight;
      if (sizes.$image1?.length) {
        const boundsInfoAfterWidth = container.querySelector('.bounds-info-after .bounds-info-width');
        boundsInfoAfterWidth.textContent = `${sizes.$image1[0].naturalWidth}px`;
        if (widthChanged) boundsInfoAfterWidth.classList.add('green');

        const boundsInfoAfterHeight = container.querySelector('.bounds-info-after .bounds-info-height');
        boundsInfoAfterHeight.textContent = `${sizes.$image1[0].naturalHeight}px`;
        if (heightChanged) boundsInfoAfterHeight.classList.add('green');
      }

      if (sizes.$image2?.length) {
        const boundsInfoBeforeWidth = container.querySelector('.bounds-info-before .bounds-info-width');
        boundsInfoBeforeWidth.textContent = `${sizes.$image2[0].naturalWidth}px`;
        if (widthChanged) boundsInfoBeforeWidth.classList.add('red');

        const boundsInfoBeforeHeight = container.querySelector('.bounds-info-before .bounds-info-height');
        boundsInfoBeforeHeight.textContent = `${sizes.$image2[0].naturalHeight}px`;
        if (heightChanged) boundsInfoBeforeHeight.classList.add('red');
      }

      const image1 = sizes.$image1[0];
      if (image1) {
        const container = image1.parentNode;
        image1.style.width = `${sizes.size1.width * factor}px`;
        image1.style.height = `${sizes.size1.height * factor}px`;
        container.style.margin = '10px auto';
        container.style.width = `${sizes.size1.width * factor + 2}px`;
        container.style.height = `${sizes.size1.height * factor + 2}px`;
      }

      const image2 = sizes.$image2[0];
      if (image2) {
        const container = image2.parentNode;
        image2.style.width = `${sizes.size2.width * factor}px`;
        image2.style.height = `${sizes.size2.height * factor}px`;
        container.style.margin = '10px auto';
        container.style.width = `${sizes.size2.width * factor + 2}px`;
        container.style.height = `${sizes.size2.height * factor + 2}px`;
      }
    }

    function initSwipe(sizes) {
      let factor = 1;
      if (sizes.max.width > diffContainerWidth - 12) {
        factor = (diffContainerWidth - 12) / sizes.max.width;
      }

      const image1 = sizes.$image1[0];
      if (image1) {
        const container = image1.parentNode;
        const swipeFrame = container.parentNode;
        image1.style.width = `${sizes.size1.width * factor}px`;
        image1.style.height = `${sizes.size1.height * factor}px`;
        container.style.margin = `0px ${sizes.ratio[0] * factor}px`;
        container.style.width = `${sizes.size1.width * factor + 2}px`;
        container.style.height = `${sizes.size1.height * factor + 2}px`;
        swipeFrame.style.padding = `${sizes.ratio[1] * factor}px 0 0 0`;
        swipeFrame.style.width = `${sizes.max.width * factor + 2}px`;
      }

      const image2 = sizes.$image2[0];
      if (image2) {
        const container = image2.parentNode;
        const swipeFrame = container.parentNode;
        image2.style.width = `${sizes.size2.width * factor}px`;
        image2.style.height = `${sizes.size2.height * factor}px`;
        container.style.margin = `${sizes.ratio[3] * factor}px ${sizes.ratio[2] * factor}px`;
        container.style.width = `${sizes.size2.width * factor + 2}px`;
        container.style.height = `${sizes.size2.height * factor + 2}px`;
        swipeFrame.style.width = `${sizes.max.width * factor + 2}px`;
        swipeFrame.style.height = `${sizes.max.height * factor + 2}px`;
      }

      // extra height for inner "position: absolute" elements
      const swipe = $container.find('.diff-swipe')[0];
      if (swipe) {
        swipe.style.width = `${sizes.max.width * factor + 2}px`;
        swipe.style.height = `${sizes.max.height * factor + 30}px`;
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
      if (sizes.max.width > diffContainerWidth - 12) {
        factor = (diffContainerWidth - 12) / sizes.max.width;
      }

      const image1 = sizes.$image1[0];
      if (image1) {
        const container = image1.parentNode;
        image1.style.width = `${sizes.size1.width * factor}px`;
        image1.style.height = `${sizes.size1.height * factor}px`;
        container.style.margin = `${sizes.ratio[1] * factor}px ${sizes.ratio[0] * factor}px`;
        container.style.width = `${sizes.size1.width * factor + 2}px`;
        container.style.height = `${sizes.size1.height * factor + 2}px`;
      }

      const image2 = sizes.$image2[0];
      if (image2) {
        const container = image2.parentNode;
        const overlayFrame = container.parentNode;
        image2.style.width = `${sizes.size2.width * factor}px`;
        image2.style.height = `${sizes.size2.height * factor}px`;
        container.style.margin = `${sizes.ratio[3] * factor}px ${sizes.ratio[2] * factor}px`;
        container.style.width = `${sizes.size2.width * factor + 2}px`;
        container.style.height = `${sizes.size2.height * factor + 2}px`;

        // some inner elements are `position: absolute`, so the container's height must be large enough
        overlayFrame.style.width = `${sizes.max.width * factor + 2}px`;
        overlayFrame.style.height = `${sizes.max.height * factor + 2}px`;
      }

      const rangeInput = $container[0].querySelector('input[type="range"]');
      function updateOpacity() {
        if (sizes?.$image1?.[0]) {
          sizes.$image1[0].parentNode.style.opacity = `${rangeInput.value / 100}`;
        }
      }
      rangeInput?.addEventListener('input', updateOpacity);
      updateOpacity();
    }
  });
}
