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
        height: img.height
      };
    }
    if (svg.hasAttribute('viewBox')) {
      const viewBox = svg.viewBox.baseVal;
      return {
        width: DefaultSize,
        height: DefaultSize * viewBox.width / viewBox.height
      };
    }
    return {
      width: DefaultSize,
      height: DefaultSize
    };
  }
  return null;
}

export function initImageDiff() {
  function createContext(image1, image2) {
    const size1 = {
      width: image1 && image1.width || 0,
      height: image1 && image1.height || 0
    };
    const size2 = {
      width: image2 && image2.width || 0,
      height: image2 && image2.height || 0
    };
    const max = {
      width: Math.max(size2.width, size1.width),
      height: Math.max(size2.height, size1.height)
    };

    return {
      image1: $(image1),
      image2: $(image2),
      size1,
      size2,
      max,
      ratio: [
        Math.floor(max.width - size1.width) / 2,
        Math.floor(max.height - size1.height) / 2,
        Math.floor(max.width - size2.width) / 2,
        Math.floor(max.height - size2.height) / 2
      ]
    };
  }

  $('.image-diff:not([data-image-diff-loaded])').each(async function() {
    const $container = $(this);
    $container.attr('data-image-diff-loaded', 'true');

    // the container may be hidden by "viewed" checkbox, so use the parent's width for reference
    const diffContainerWidth = Math.max($container.closest('.diff-file-box').width() - 300, 100);

    const imageInfos = [{
      path: this.getAttribute('data-path-after'),
      mime: this.getAttribute('data-mime-after'),
      $images: $container.find('img.image-after'), // matches 3 <img>
      $boundsInfo: $container.find('.bounds-info-after')
    }, {
      path: this.getAttribute('data-path-before'),
      mime: this.getAttribute('data-mime-before'),
      $images: $container.find('img.image-before'), // matches 3 <img>
      $boundsInfo: $container.find('.bounds-info-before')
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
          info.$images.attr('width', bounds.width);
          info.$images.attr('height', bounds.height);
          hideElem(info.$boundsInfo);
        }
      }
    }));

    const $imagesAfter = imageInfos[0].$images;
    const $imagesBefore = imageInfos[1].$images;

    initSideBySide(createContext($imagesAfter[0], $imagesBefore[0]));
    if ($imagesAfter.length > 0 && $imagesBefore.length > 0) {
      initSwipe(createContext($imagesAfter[1], $imagesBefore[1]));
      initOverlay(createContext($imagesAfter[2], $imagesBefore[2]));
    }

    $container.find('> .image-diff-tabs').removeClass('is-loading');

    function initSideBySide(sizes) {
      let factor = 1;
      if (sizes.max.width > (diffContainerWidth - 24) / 2) {
        factor = (diffContainerWidth - 24) / 2 / sizes.max.width;
      }

      const widthChanged = sizes.image1.length !== 0 && sizes.image2.length !== 0 && sizes.image1[0].naturalWidth !== sizes.image2[0].naturalWidth;
      const heightChanged = sizes.image1.length !== 0 && sizes.image2.length !== 0 && sizes.image1[0].naturalHeight !== sizes.image2[0].naturalHeight;
      if (sizes.image1.length !== 0) {
        $container.find('.bounds-info-after .bounds-info-width').text(`${sizes.image1[0].naturalWidth}px`).addClass(widthChanged ? 'green' : '');
        $container.find('.bounds-info-after .bounds-info-height').text(`${sizes.image1[0].naturalHeight}px`).addClass(heightChanged ? 'green' : '');
      }
      if (sizes.image2.length !== 0) {
        $container.find('.bounds-info-before .bounds-info-width').text(`${sizes.image2[0].naturalWidth}px`).addClass(widthChanged ? 'red' : '');
        $container.find('.bounds-info-before .bounds-info-height').text(`${sizes.image2[0].naturalHeight}px`).addClass(heightChanged ? 'red' : '');
      }

      sizes.image1.css({
        width: sizes.size1.width * factor,
        height: sizes.size1.height * factor
      });
      sizes.image1.parent().css({
        margin: `10px auto`,
        width: sizes.size1.width * factor + 2,
        height: sizes.size1.height * factor + 2
      });
      sizes.image2.css({
        width: sizes.size2.width * factor,
        height: sizes.size2.height * factor
      });
      sizes.image2.parent().css({
        margin: `10px auto`,
        width: sizes.size2.width * factor + 2,
        height: sizes.size2.height * factor + 2
      });
    }

    function initSwipe(sizes) {
      let factor = 1;
      if (sizes.max.width > diffContainerWidth - 12) {
        factor = (diffContainerWidth - 12) / sizes.max.width;
      }

      sizes.image1.css({
        width: sizes.size1.width * factor,
        height: sizes.size1.height * factor
      });
      sizes.image1.parent().css({
        margin: `0px ${sizes.ratio[0] * factor}px`,
        width: sizes.size1.width * factor + 2,
        height: sizes.size1.height * factor + 2
      });
      sizes.image1.parent().parent().css({
        padding: `${sizes.ratio[1] * factor}px 0 0 0`,
        width: sizes.max.width * factor + 2
      });
      sizes.image2.css({
        width: sizes.size2.width * factor,
        height: sizes.size2.height * factor
      });
      sizes.image2.parent().css({
        margin: `${sizes.ratio[3] * factor}px ${sizes.ratio[2] * factor}px`,
        width: sizes.size2.width * factor + 2,
        height: sizes.size2.height * factor + 2
      });
      sizes.image2.parent().parent().css({
        width: sizes.max.width * factor + 2,
        height: sizes.max.height * factor + 2
      });
      $container.find('.diff-swipe').css({
        width: sizes.max.width * factor + 2,
        height: sizes.max.height * factor + 30 /* extra height for inner "position: absolute" elements */,
      });
      $container.find('.swipe-bar').on('mousedown', function(e) {
        e.preventDefault();

        const $swipeBar = $(this);
        const $swipeFrame = $swipeBar.parent();
        const width = $swipeFrame.width() - $swipeBar.width() - 2;

        $(document).on('mousemove.diff-swipe', (e2) => {
          e2.preventDefault();

          const value = Math.max(0, Math.min(e2.clientX - $swipeFrame.offset().left, width));

          $swipeBar.css({
            left: value
          });
          $container.find('.swipe-container').css({
            width: $swipeFrame.width() - value
          });
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

      sizes.image1.css({
        width: sizes.size1.width * factor,
        height: sizes.size1.height * factor
      });
      sizes.image2.css({
        width: sizes.size2.width * factor,
        height: sizes.size2.height * factor
      });
      sizes.image1.parent().css({
        margin: `${sizes.ratio[1] * factor}px ${sizes.ratio[0] * factor}px`,
        width: sizes.size1.width * factor + 2,
        height: sizes.size1.height * factor + 2
      });
      sizes.image2.parent().css({
        margin: `${sizes.ratio[3] * factor}px ${sizes.ratio[2] * factor}px`,
        width: sizes.size2.width * factor + 2,
        height: sizes.size2.height * factor + 2
      });

      // some inner elements are `position: absolute`, so the container's height must be large enough
      // the "css(width, height)" is somewhat hacky and not easy to understand, it could be improved in the future
      sizes.image2.parent().parent().css({
        width: sizes.max.width * factor + 2,
        height: sizes.max.height * factor + 2,
      });

      const $range = $container.find("input[type='range']");
      const onInput = () => sizes.image1.parent().css({
        opacity: $range.val() / 100
      });
      $range.on('input', onInput);
      onInput();
    }
  });
}
