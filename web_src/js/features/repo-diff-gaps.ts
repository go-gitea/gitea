import {reactive} from 'vue';
import {svg, type SvgName} from '../svg.ts';
import {html, htmlRaw} from '../utils/html.ts';

// A "gap" is a stretch of unchanged lines the diff hides behind a section row. The backend renders
// the row with the gap's own line numbers; everything below works them out from there, so expanding
// never needs the server to say what is left.
//
// Must match gitdiff.BlobExcerptChunkSize.
export const blobExcerptChunkSize = 20;

export type DiffGap = {
  key: string; // identifies the gap for the life of the page, even as it shrinks
  anchor: string;
  lastLeft: number; lastRight: number; // the last line rendered before the gap
  left: number; right: number; // the next line rendered after it, or the file's last line at its end
  leftHunk: number; rightHunk: number; // zero on both sides means the gap runs to the end of the file
};

// the gaps of every file on the page, keyed by "<file box id>:<gap key>"
const diffGaps = reactive(new Map<string, DiffGap>());

export function parseDiffGap(el: Element): DiffGap {
  const [lastLeft, lastRight, left, right, leftHunk, rightHunk] = el.getAttribute('data-gap')!.split(',').map(Number);
  return {key: el.getAttribute('data-gap-key')!, anchor: el.getAttribute('data-gap-anchor')!, lastLeft, lastRight, left, right, leftHunk, rightHunk};
}

function gapStoreKey(el: Element, gapKey: string): string {
  return `${el.closest('.diff-file-box')?.id ?? ''}:${gapKey}`;
}

export function getDiffGap(el: Element, gapKey: string): DiffGap | undefined {
  return diffGaps.get(gapStoreKey(el, gapKey));
}

export function setDiffGap(el: Element, gap: DiffGap) {
  diffGaps.set(gapStoreKey(el, gap.key), gap);
}

// runs to the end of the file, so nothing follows the gap and "right" is its last line
export function gapReachesFileEnd(gap: DiffGap): boolean {
  return gap.leftHunk <= 0 && gap.rightHunk <= 0;
}

// which arrows a gap deserves, mirroring gitdiff.GetExpandDirection
export function gapExpandDirection(gap: DiffGap): '' | 'up' | 'down' | 'updown' | 'single' {
  if (gap.left - gap.lastLeft <= 1 || gap.right - gap.lastRight <= 1) return '';
  if (gap.lastLeft <= 0 && gap.lastRight <= 0) return 'up';
  if (gap.right - gap.lastRight - 1 > blobExcerptChunkSize && gap.rightHunk > 0) return 'updown';
  if (gapReachesFileEnd(gap)) return 'down';
  return 'single';
}

export function gapExcerptUrl(baseUrl: string, gap: DiffGap, direction: string): string {
  const url = new URL(baseUrl, window.location.href);
  url.searchParams.set('direction', direction);
  url.searchParams.set('anchor', gap.anchor);
  url.searchParams.set('gap_key', gap.key);
  url.searchParams.set('last_left', String(gap.lastLeft));
  url.searchParams.set('last_right', String(gap.lastRight));
  url.searchParams.set('left', String(gap.left));
  url.searchParams.set('right', String(gap.right));
  url.searchParams.set('left_hunk_size', String(gap.leftHunk));
  url.searchParams.set('right_hunk_size', String(gap.rightHunk));
  return url.href;
}

const arrowSvgs: Record<string, SvgName> = {up: 'octicon-fold-up', down: 'octicon-fold-down', single: 'octicon-fold'};

// the arrows are rendered from the gap, so a gap that shrank shows the arrows it now deserves
export function renderGapExpander(elExpander: Element, gap: DiffGap) {
  const direction = gapExpandDirection(gap);
  elExpander.setAttribute('data-expand-direction', direction);
  const directions = direction === 'updown' ? ['down', 'up'] : (direction ? [direction] : []);
  const arrows = directions.map((d) => html`<button class="code-expander-button" data-global-click="diffExpandHiddenLines" data-gap-direction="${d}">${htmlRaw(svg(arrowSvgs[d]))}</button>`);
  for (const el of elExpander.querySelectorAll('.code-expander-button')) el.remove();
  elExpander.insertAdjacentHTML('beforeend', arrows.join(''));
}
