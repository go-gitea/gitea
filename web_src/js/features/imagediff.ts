import {GET} from '../modules/fetch.ts';
import {hideElem, loadElem, queryElemChildren, queryElems} from '../utils/dom.ts';
import {parseDom} from '../utils.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

function getDefaultSvgBoundsIfUndefined(text, src) {
  const defaultSize = 300;
  const maxSize = 99999;

  const svgDoc = parseDom(text, 'image/svg+xml');
  const svg = (svgDoc.documentElement as unknown) as SVGSVGElement;
  const width = svg?.width?.baseVal;
  const height = svg?.height?.baseVal;
  if (width === undefined || height === undefined) {
    return null; // in case some svg is invalid or doesn't have the width/height
  }
  if (width.unitType === SVGLength.SVG_LENGTHTYPE_PERCENTAGE || height.unitType === SVGLength.SVG_LENGTHTYPE_PERCENTAGE) {
    const img = new Image();
    img.src = src;
    if (img.width > 1 && img.width < maxSize && img.height > 1 && img.height < maxSize) {
      return {
        width: img.width,
        height: img.height,
      };
    }
    if (svg.hasAttribute('viewBox')) {
      const viewBox = svg.viewBox.baseVal;
      return {
        width: defaultSize,
        height: defaultSize * viewBox.width / viewBox.height,
      };
    }
    return {
      width: defaultSize,
      height: defaultSize,
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

class ImageDiff {
  containerEl: HTMLElement;
  diffContainerWidth: number;

  async init(containerEl: HTMLElement) {
    this.containerEl = containerEl;
    containerEl.setAttribute('data-image-diff-loaded', 'true');

    fomanticQuery(containerEl).find('.ui.menu.tabular .item').tab({autoTabActivation: false});

    // the container may be hidden by "viewed" checkbox, so use the parent's width for reference
    this.diffContainerWidth = Math.max(containerEl.closest('.diff-file-box').clientWidth - 300, 100);

    const imageInfos = [{
      path: containerEl.getAttribute('data-path-after'),
      mime: containerEl.getAttribute('data-mime-after'),
      images: containerEl.querySelectorAll<HTMLImageElement>('img.image-after'), // matches 3 <img>
      boundsInfo: containerEl.querySelector('.bounds-info-after'),
    }, {
      path: containerEl.getAttribute('data-path-before'),
      mime: containerEl.getAttribute('data-mime-before'),
      images: containerEl.querySelectorAll<HTMLImageElement>('img.image-before'), // matches 3 <img>
      boundsInfo: containerEl.querySelector('.bounds-info-before'),
    }];

    await Promise.all(imageInfos.map(async (info) => {
      const [success] = await Promise.all(Array.from(info.images, (img) => {
        return loadElem(img, info.path);
      }));
      // only the first images is associated with boundsInfo
      if (!success && info.boundsInfo) info.boundsInfo.textContent = '(image error)';
      if (info.mime === 'image/svg+xml') {
        const resp = await GET(info.path);
        const text = await resp.text();
        const bounds = getDefaultSvgBoundsIfUndefined(text, info.path);
        if (bounds) {
          for (const el of info.images) {
            el.setAttribute('width', String(bounds.width));
            el.setAttribute('height', String(bounds.height));
          }
          hideElem(info.boundsInfo);
        }
      }
    }));

    const imagesAfter = imageInfos[0].images;
    const imagesBefore = imageInfos[1].images;

    this.initSideBySide(createContext(imagesAfter[0], imagesBefore[0]));
    if (imagesAfter.length > 0 && imagesBefore.length > 0) {
      this.initSwipe(createContext(imagesAfter[1], imagesBefore[1]));
      this.initOverlay(createContext(imagesAfter[2], imagesBefore[2]));
    }
    queryElemChildren(containerEl, '.image-diff-tabs', (el) => el.classList.remove('is-loading'));
  }

  initSideBySide(sizes) {
    let factor = 1;
    if (sizes.maxSize.width > (this.diffContainerWidth - 24) / 2) {
      factor = (this.diffContainerWidth - 24) / 2 / sizes.maxSize.width;
    }

    const widthChanged = sizes.imageAfter && sizes.imageBefore && sizes.imageAfter.naturalWidth !== sizes.imageBefore.naturalWidth;
    const heightChanged = sizes.imageAfter && sizes.imageBefore && sizes.imageAfter.naturalHeight !== sizes.imageBefore.naturalHeight;
    if (sizes.imageAfter) {
      const boundsInfoAfterWidth = this.containerEl.querySelector('.bounds-info-after .bounds-info-width');
      if (boundsInfoAfterWidth) {
        boundsInfoAfterWidth.textContent = `${sizes.imageAfter.naturalWidth}px`;
        boundsInfoAfterWidth.classList.toggle('green', widthChanged);
      }
      const boundsInfoAfterHeight = this.containerEl.querySelector('.bounds-info-after .bounds-info-height');
      if (boundsInfoAfterHeight) {
        boundsInfoAfterHeight.textContent = `${sizes.imageAfter.naturalHeight}px`;
        boundsInfoAfterHeight.classList.toggle('green', heightChanged);
      }
    }

    if (sizes.imageBefore) {
      const boundsInfoBeforeWidth = this.containerEl.querySelector('.bounds-info-before .bounds-info-width');
      if (boundsInfoBeforeWidth) {
        boundsInfoBeforeWidth.textContent = `${sizes.imageBefore.naturalWidth}px`;
        boundsInfoBeforeWidth.classList.toggle('red', widthChanged);
      }
      const boundsInfoBeforeHeight = this.containerEl.querySelector('.bounds-info-before .bounds-info-height');
      if (boundsInfoBeforeHeight) {
        boundsInfoBeforeHeight.textContent = `${sizes.imageBefore.naturalHeight}px`;
        boundsInfoBeforeHeight.classList.toggle('red', heightChanged);
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

  initSwipe(sizes) {
    let factor = 1;
    if (sizes.maxSize.width > this.diffContainerWidth - 12) {
      factor = (this.diffContainerWidth - 12) / sizes.maxSize.width;
    }

    if (sizes.imageAfter) {
      const imgParent = sizes.imageAfter.parentNode;
      const swipeFrame = imgParent.parentNode;
      sizes.imageAfter.style.width = `${sizes.sizeAfter.width * factor}px`;
      sizes.imageAfter.style.height = `${sizes.sizeAfter.height * factor}px`;
      imgParent.style.margin = `0px ${sizes.ratio[0] * factor}px`;
      imgParent.style.width = `${sizes.sizeAfter.width * factor + 2}px`;
      imgParent.style.height = `${sizes.sizeAfter.height * factor + 2}px`;
      swipeFrame.style.padding = `${sizes.ratio[1] * factor}px 0 0 0`;
      swipeFrame.style.width = `${sizes.maxSize.width * factor + 2}px`;
    }

    if (sizes.imageBefore) {
      const imgParent = sizes.imageBefore.parentNode;
      const swipeFrame = imgParent.parentNode;
      sizes.imageBefore.style.width = `${sizes.sizeBefore.width * factor}px`;
      sizes.imageBefore.style.height = `${sizes.sizeBefore.height * factor}px`;
      imgParent.style.margin = `${sizes.ratio[3] * factor}px ${sizes.ratio[2] * factor}px`;
      imgParent.style.width = `${sizes.sizeBefore.width * factor + 2}px`;
      imgParent.style.height = `${sizes.sizeBefore.height * factor + 2}px`;
      swipeFrame.style.width = `${sizes.maxSize.width * factor + 2}px`;
      swipeFrame.style.height = `${sizes.maxSize.height * factor + 2}px`;
    }

    // extra height for inner "position: absolute" elements
    const swipe = this.containerEl.querySelector<HTMLElement>('.diff-swipe');
    if (swipe) {
      swipe.style.width = `${sizes.maxSize.width * factor + 2}px`;
      swipe.style.height = `${sizes.maxSize.height * factor + 30}px`;
    }

    this.containerEl.querySelector('.swipe-bar').addEventListener('mousedown', (e) => {
      e.preventDefault();
      this.initSwipeEventListeners(e.currentTarget);
    });
  }

  initSwipeEventListeners(swipeBar) {
    const swipeFrame = swipeBar.parentNode;
    const width = swipeFrame.clientWidth;
    const onSwipeMouseMove = (e) => {
      e.preventDefault();
      const rect = swipeFrame.getBoundingClientRect();
      const value = Math.max(0, Math.min(e.clientX - rect.left, width));
      swipeBar.style.left = `${value}px`;
      this.containerEl.querySelector<HTMLElement>('.swipe-container').style.width = `${swipeFrame.clientWidth - value}px`;
    };
    const removeEventListeners = () => {
      document.removeEventListener('mousemove', onSwipeMouseMove);
      document.removeEventListener('mouseup', removeEventListeners);
    };
    document.addEventListener('mousemove', onSwipeMouseMove);
    document.addEventListener('mouseup', removeEventListeners);
  }

  initOverlay(sizes) {
    let factor = 1;
    if (sizes.maxSize.width > this.diffContainerWidth - 12) {
      factor = (this.diffContainerWidth - 12) / sizes.maxSize.width;
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

    const rangeInput = this.containerEl.querySelector<HTMLInputElement>('input[type="range"]');

    function updateOpacity() {
      if (sizes.imageAfter) {
        sizes.imageAfter.parentNode.style.opacity = `${Number(rangeInput.value) / 100}`;
      }
    }

    rangeInput?.addEventListener('input', updateOpacity);
    updateOpacity();
  }
}

export function initImageDiff() {
  for (const el of queryElems<HTMLImageElement>(document, '.image-diff:not([data-image-diff-loaded])')) {
    (new ImageDiff()).init(el); // it is async, but we don't need to await for it
  }
}
