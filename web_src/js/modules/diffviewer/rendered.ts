import {html, htmlRaw} from '../../utils/html.ts';
import {createElementFromAttrs} from '../../utils/dom.ts';
import {extname, formatBytes} from '../../utils.ts';
import {trString} from '../i18n.ts';
import {CsvParseError, createCsvDiff, determineDelimiter, parseCsv, renderCsvDiff} from './csvdiff.ts';
import {fileUrl} from './content.ts';
import type {DiffFile} from './entry.ts';
import type {DiffViewerConfig, DiffViewerLocale} from './types.ts';

type ImageSide = {path: string; mime: string; size: number} | null;

// the kind of view shown besides the source
export function renderedKind(entry: DiffFile, config: DiffViewerConfig): 'image' | 'csv' | null {
  const {file} = entry;
  if (!config.repoLink || entry.missing || file.tooLarge || file.submodule) return null; // the wiki can't load the files
  if (file.oldImage || file.newImage) return 'image';
  const ext = extname(file.name).toLowerCase();
  if (!file.isBinary && !file.lfs && (ext === '.csv' || ext === '.tsv')) return 'csv';
  return null;
}

function imageSides(entry: DiffFile, config: DiffViewerConfig): [ImageSide, ImageSide] {
  const {file} = entry;
  const view = file.lfs ? 'media' : 'raw';
  return [
    file.oldImage ? {path: fileUrl(config, view, 'old', file.oldName), mime: file.oldImage, size: file.lfs?.old?.size ?? file.oldSize ?? 0} : null,
    file.newImage ? {path: fileUrl(config, view, 'new', file.name), mime: file.newImage, size: file.lfs?.new?.size ?? file.newSize ?? 0} : null,
  ];
}

// the global init "initDiffFileImageDiff" loads the images
export function imageDiffHtml(entry: DiffFile, config: DiffViewerConfig, locale: DiffViewerLocale): string {
  const {nameHash} = entry.file;
  const [before, after] = imageSides(entry, config);
  const both = before !== null && after !== null;
  const sideHtml = (side: 'before' | 'after', image: ImageSide) => (image ? html`
    <span class="side">
      <p class="side-header">${side === 'before' ? locale.fileBefore : locale.fileAfter}</p>
      <span class="${side}-container"><img alt class="image-${side}"></span>
      <p>
        <span class="bounds-info-${side}">
          ${locale.imageWidth}: <span class="text bounds-info-width"></span>
          &nbsp;|&nbsp;
          ${locale.imageHeight}: <span class="text bounds-info-height"></span>
          &nbsp;|&nbsp;
        </span>
        ${locale.byteSize}: <span class="text">${formatBytes(image.size)}</span>
      </p>
    </span>
  ` : '');
  const tabs = both ? html`
    <a class="item" data-tab="diff-swipe-${nameHash}">${locale.swipe}</a>
    <a class="item" data-tab="diff-overlay-${nameHash}">${locale.overlay}</a>
  ` : '';
  const swipeAndOverlay = both ? html`
    <div class="ui bottom attached tab image-diff-container" data-tab="diff-swipe-${nameHash}">
      <div class="diff-swipe">
        <div class="swipe-frame">
          <span class="before-container"><img alt class="image-before"></span>
          <span class="swipe-container"><span class="after-container"><img alt class="image-after"></span></span>
          <span class="swipe-bar"><span class="handle top-handle"></span><span class="handle bottom-handle"></span></span>
        </div>
      </div>
    </div>
    <div class="ui bottom attached tab image-diff-container" data-tab="diff-overlay-${nameHash}">
      <div class="diff-overlay">
        <input type="range" min="0" max="100" value="50">
        <div class="overlay-frame">
          <span class="before-container"><img alt class="image-before"></span>
          <span class="after-container"><img alt class="image-after"></span>
        </div>
      </div>
    </div>
  ` : '';
  return html`
    <table class="chroma tw-w-full"><tr><td colspan="2">
      <div class="image-diff" data-global-init="initDiffFileImageDiff"
        data-path-before="${before?.path ?? ''}" data-path-after="${after?.path ?? ''}"
        data-mime-before="${before?.mime ?? ''}" data-mime-after="${after?.mime ?? ''}">
        <overflow-menu class="ui secondary pointing tabular menu">
          <div class="overflow-menu-items tw-justify-center" data-global-init="initTabSwitcher">
            <a class="item active" data-tab="diff-side-by-side-${nameHash}">${locale.sideBySide}</a>
            ${htmlRaw(tabs)}
          </div>
        </overflow-menu>
        <div class="image-diff-tabs is-loading">
          <div class="ui bottom attached tab image-diff-container active" data-tab="diff-side-by-side-${nameHash}">
            <div class="diff-side-by-side">${htmlRaw(sideHtml('before', before))}${htmlRaw(sideHtml('after', after))}</div>
          </div>
          ${htmlRaw(swipeAndOverlay)}
        </div>
      </div>
    </td></tr></table>
  `;
}

export async function renderCsvView(entry: DiffFile, container: HTMLElement, loadContent: () => Promise<{old: string | null; new: string | null}>,
  maxFileSize: number, locale: DiffViewerLocale): Promise<void> {
  const message = (text: string) => container.replaceChildren(createElementFromAttrs('div', {class: 'ui center'}, text));
  const {file} = entry;
  if (maxFileSize > 0 && Math.max(file.oldSize ?? 0, file.newSize ?? 0) > maxFileSize) {
    message(locale.csvTooLarge);
    return;
  }
  container.classList.add('is-loading');
  try {
    const content = await loadContent();
    const rows = (text: string | null, name: string) => (text === null ? null : parseCsv(text, determineDelimiter(name, text)));
    const sections = createCsvDiff(rows(content.old, file.oldName), rows(content.new, file.name), entry.hunks);
    container.innerHTML = html`<table class="chroma tw-w-full"><tr><td>${htmlRaw(renderCsvDiff(sections))}</td></tr></table>`;
  } catch (error) {
    if (!(error instanceof CsvParseError)) {
      message(window.config.i18n.error_occurred);
      throw error;
    }
    message(error.fieldCount ? trString(locale.csvFieldCount, error.line) : trString(locale.csvUnexpected, error.line, error.column));
  } finally {
    container.classList.remove('is-loading');
  }
}
