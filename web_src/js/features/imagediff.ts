import {GET} from '../modules/fetch.ts';
import {hideElem, loadElem, queryElemChildren, queryElems} from '../utils/dom.ts';
import {parseDom} from '../utils.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

type ImageContext = {
  imageBefore: HTMLImageElement | undefined,
  imageAfter: HTMLImageElement | undefined,
  sizeBefore: {width: number, height: number},
  sizeAfter: {width: number, height: number},
  maxSize: {width: number, height: number},
  ratio: [number, number, number, number],
};

type ImageInfo = {
  path: string | null,
  mime: string | null,
  images: NodeListOf<HTMLImageElement>,
  boundsInfo: HTMLElement | null,
};

type Bounds = {
  width: number,
  height: number,
} | null;

type SvgBoundsInfo = {
  before: Bounds,
  after: Bounds,
};

function getDefaultSvgBoundsIfUndefined(text: string, src: string): Bounds | null {
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

function createContext(imageAfter: HTMLImageElement, imageBefore: HTMLImageElement, svgBoundsInfo: SvgBoundsInfo): ImageContext {
  const sizeAfter = {
    width: svgBoundsInfo.after?.width || imageAfter?.width || 0,
    height: svgBoundsInfo.after?.height || imageAfter?.height || 0,
  };
  const sizeBefore = {
    width: svgBoundsInfo.before?.width || imageBefore?.width || 0,
    height: svgBoundsInfo.before?.height || imageBefore?.height || 0,
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

    fomanticQuery(containerEl).find('.ui.menu.tabular .item').tab();

    // the container may be hidden by "viewed" checkbox, so use the parent's width for reference
    this.diffContainerWidth = Math.max(containerEl.closest('.diff-file-box')!.clientWidth - 300, 100);

    const imagePair: [ImageInfo, ImageInfo] = [{
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

    const svgBoundsInfo: SvgBoundsInfo = {before: null, after: null};
    await Promise.all(imagePair.map(async (info, index) => {
      const [success] = await Promise.all(Array.from(info.images, (img) => {
        return loadElem(img, info.path!);
      }));
      // only the first images is associated with boundsInfo
      if (!success && info.boundsInfo) info.boundsInfo.textContent = '(image error)';
      if (info.mime === 'image/svg+xml') {
        const resp = await GET(info.path!);
        const text = await resp.text();
        const bounds = getDefaultSvgBoundsIfUndefined(text, info.path!);
        svgBoundsInfo[index === 0 ? 'after' : 'before'] = bounds;
        if (bounds) {
          hideElem(info.boundsInfo!);
        }
      }
    }));

    const imagesAfter = imagePair[0].images;
    const imagesBefore = imagePair[1].images;

    this.initSideBySide(createContext(imagesAfter[0], imagesBefore[0], svgBoundsInfo));
    if (imagesAfter.length > 0 && imagesBefore.length > 0) {
      this.initSwipe(createContext(imagesAfter[1], imagesBefore[1], svgBoundsInfo));
      this.initOverlay(createContext(imagesAfter[2], imagesBefore[2], svgBoundsInfo));
    }
    queryElemChildren(containerEl, '.image-diff-tabs', (el) => el.classList.remove('is-loading'));
  }

  initSideBySide(ctx: ImageContext) {
    let factor = 1;
    if (ctx.maxSize.width > (this.diffContainerWidth - 24) / 2) {
      factor = (this.diffContainerWidth - 24) / 2 / ctx.maxSize.width;
    }

    const widthChanged = ctx.imageAfter && ctx.imageBefore && ctx.imageAfter.naturalWidth !== ctx.imageBefore.naturalWidth;
    const heightChanged = ctx.imageAfter && ctx.imageBefore && ctx.imageAfter.naturalHeight !== ctx.imageBefore.naturalHeight;
    if (ctx.imageAfter) {
      const boundsInfoAfterWidth = this.containerEl.querySelector('.bounds-info-after .bounds-info-width');
      if (boundsInfoAfterWidth) {
        boundsInfoAfterWidth.textContent = `${ctx.imageAfter.naturalWidth}px`;
        boundsInfoAfterWidth.classList.toggle('green', widthChanged);
      }
      const boundsInfoAfterHeight = this.containerEl.querySelector('.bounds-info-after .bounds-info-height');
      if (boundsInfoAfterHeight) {
        boundsInfoAfterHeight.textContent = `${ctx.imageAfter.naturalHeight}px`;
        boundsInfoAfterHeight.classList.toggle('green', heightChanged);
      }
    }

    if (ctx.imageBefore) {
      const boundsInfoBeforeWidth = this.containerEl.querySelector('.bounds-info-before .bounds-info-width');
      if (boundsInfoBeforeWidth) {
        boundsInfoBeforeWidth.textContent = `${ctx.imageBefore.naturalWidth}px`;
        boundsInfoBeforeWidth.classList.toggle('red', widthChanged);
      }
      const boundsInfoBeforeHeight = this.containerEl.querySelector('.bounds-info-before .bounds-info-height');
      if (boundsInfoBeforeHeight) {
        boundsInfoBeforeHeight.textContent = `${ctx.imageBefore.naturalHeight}px`;
        boundsInfoBeforeHeight.classList.toggle('red', heightChanged);
      }
    }

    if (ctx.imageAfter) {
      const container = ctx.imageAfter.parentNode as HTMLElement;
      ctx.imageAfter.style.width = `${ctx.sizeAfter.width * factor}px`;
      ctx.imageAfter.style.height = `${ctx.sizeAfter.height * factor}px`;
      container.style.margin = '10px auto';
      container.style.width = `${ctx.sizeAfter.width * factor + 2}px`;
      container.style.height = `${ctx.sizeAfter.height * factor + 2}px`;
    }

    if (ctx.imageBefore) {
      const container = ctx.imageBefore.parentNode as HTMLElement;
      ctx.imageBefore.style.width = `${ctx.sizeBefore.width * factor}px`;
      ctx.imageBefore.style.height = `${ctx.sizeBefore.height * factor}px`;
      container.style.margin = '10px auto';
      container.style.width = `${ctx.sizeBefore.width * factor + 2}px`;
      container.style.height = `${ctx.sizeBefore.height * factor + 2}px`;
    }
  }

  initSwipe(ctx: ImageContext) {
    let factor = 1;
    if (ctx.maxSize.width > this.diffContainerWidth - 12) {
      factor = (this.diffContainerWidth - 12) / ctx.maxSize.width;
    }

    if (ctx.imageAfter) {
      const imgParent = ctx.imageAfter.parentNode as HTMLElement;
      const swipeFrame = imgParent.parentNode as HTMLElement;
      ctx.imageAfter.style.width = `${ctx.sizeAfter.width * factor}px`;
      ctx.imageAfter.style.height = `${ctx.sizeAfter.height * factor}px`;
      imgParent.style.margin = `0px ${ctx.ratio[0] * factor}px`;
      imgParent.style.width = `${ctx.sizeAfter.width * factor + 2}px`;
      imgParent.style.height = `${ctx.sizeAfter.height * factor + 2}px`;
      swipeFrame.style.padding = `${ctx.ratio[1] * factor}px 0 0 0`;
      swipeFrame.style.width = `${ctx.maxSize.width * factor + 2}px`;
    }

    if (ctx.imageBefore) {
      const imgParent = ctx.imageBefore.parentNode as HTMLElement;
      const swipeFrame = imgParent.parentNode as HTMLElement;
      ctx.imageBefore.style.width = `${ctx.sizeBefore.width * factor}px`;
      ctx.imageBefore.style.height = `${ctx.sizeBefore.height * factor}px`;
      imgParent.style.margin = `${ctx.ratio[3] * factor}px ${ctx.ratio[2] * factor}px`;
      imgParent.style.width = `${ctx.sizeBefore.width * factor + 2}px`;
      imgParent.style.height = `${ctx.sizeBefore.height * factor + 2}px`;
      swipeFrame.style.width = `${ctx.maxSize.width * factor + 2}px`;
      swipeFrame.style.height = `${ctx.maxSize.height * factor + 2}px`;
    }

    // extra height for inner "position: absolute" elements
    const swipe = this.containerEl.querySelector<HTMLElement>('.diff-swipe');
    if (swipe) {
      swipe.style.width = `${ctx.maxSize.width * factor + 2}px`;
      swipe.style.height = `${ctx.maxSize.height * factor + 30}px`;
    }

    this.containerEl.querySelector('.swipe-bar')!.addEventListener('mousedown', (e) => {
      e.preventDefault();
      this.initSwipeEventListeners(e.currentTarget as HTMLElement);
    });
  }

  initSwipeEventListeners(swipeBar: HTMLElement) {
    const swipeFrame = swipeBar.parentNode as HTMLElement;
    const width = swipeFrame.clientWidth;
    const onSwipeMouseMove = (e: MouseEvent) => {
      e.preventDefault();
      const rect = swipeFrame.getBoundingClientRect();
      const value = Math.max(0, Math.min(e.clientX - rect.left, width));
      swipeBar.style.left = `${value}px`;
      this.containerEl.querySelector<HTMLElement>('.swipe-container')!.style.width = `${swipeFrame.clientWidth - value}px`;
    };
    const removeEventListeners = () => {
      document.removeEventListener('mousemove', onSwipeMouseMove);
      document.removeEventListener('mouseup', removeEventListeners);
    };
    document.addEventListener('mousemove', onSwipeMouseMove);
    document.addEventListener('mouseup', removeEventListeners);
  }

  initOverlay(ctx: ImageContext) {
    let factor = 1;
    if (ctx.maxSize.width > this.diffContainerWidth - 12) {
      factor = (this.diffContainerWidth - 12) / ctx.maxSize.width;
    }

    if (ctx.imageAfter) {
      const container = ctx.imageAfter.parentNode as HTMLElement;
      ctx.imageAfter.style.width = `${ctx.sizeAfter.width * factor}px`;
      ctx.imageAfter.style.height = `${ctx.sizeAfter.height * factor}px`;
      container.style.margin = `${ctx.ratio[1] * factor}px ${ctx.ratio[0] * factor}px`;
      container.style.width = `${ctx.sizeAfter.width * factor + 2}px`;
      container.style.height = `${ctx.sizeAfter.height * factor + 2}px`;
    }

    if (ctx.imageBefore) {
      const container = ctx.imageBefore.parentNode as HTMLElement;
      const overlayFrame = container.parentNode as HTMLElement;
      ctx.imageBefore.style.width = `${ctx.sizeBefore.width * factor}px`;
      ctx.imageBefore.style.height = `${ctx.sizeBefore.height * factor}px`;
      container.style.margin = `${ctx.ratio[3] * factor}px ${ctx.ratio[2] * factor}px`;
      container.style.width = `${ctx.sizeBefore.width * factor + 2}px`;
      container.style.height = `${ctx.sizeBefore.height * factor + 2}px`;

      // some inner elements are `position: absolute`, so the container's height must be large enough
      overlayFrame.style.width = `${ctx.maxSize.width * factor + 2}px`;
      overlayFrame.style.height = `${ctx.maxSize.height * factor + 2}px`;
    }

    const rangeInput = this.containerEl.querySelector<HTMLInputElement>('input[type="range"]')!;

    function updateOpacity() {
      if (ctx.imageAfter) {
        (ctx.imageAfter.parentNode as HTMLElement).style.opacity = `${Number(rangeInput.value) / 100}`;
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
