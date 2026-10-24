import {effectScope, reactive, watchEffect} from 'vue';
import {toggleElem} from '../utils/dom.ts';

// A "gap" is a stretch of unchanged lines the diff hides behind a section row. Its rows are rendered
// by the server, so Vue only tracks the state and they are placed by hand.
export const blobExcerptChunkSize = 20; // must match gitdiff.BlobExcerptChunkSize

export type DiffGap = {
  lastLeft: number; lastRight: number; // the last line rendered before the gap
  left: number; right: number; // the next line rendered after it, or the file's last line at its end
  leftHunk: number; rightHunk: number; // zero on both sides means the gap runs to the end of the file
  hiddenCommentIds: string[]; // comments the diff does not show while this gap is closed
};

// "original" is the gap as the diff described it, "current" what is left of it, "rows" what it shows
type DiffGapState = {original: DiffGap; current: DiffGap; rows: HTMLElement[]; expanded: HTMLElement[]};

const diffGaps = reactive(new Map<string, DiffGapState>());

// a gap is its six numbers, and the ones it started with name it for the life of the page
export function gapNumbers(gap: DiffGap): string {
  return [gap.lastLeft, gap.lastRight, gap.left, gap.right, gap.leftHunk, gap.rightHunk].join(',');
}

function gapStoreKey(el: Element, gapKey: string): string {
  return `${el.closest('.diff-file-box')!.id}:${gapKey}`;
}

export function parseGapNumbers(numbers: string, hiddenCommentIds: string[]): DiffGap {
  const [lastLeft, lastRight, left, right, leftHunk, rightHunk] = numbers.split(',').map(Number);
  return {lastLeft, lastRight, left, right, leftHunk, rightHunk, hiddenCommentIds};
}

export function parseDiffGap(el: Element): DiffGap {
  return parseGapNumbers(el.getAttribute('data-gap')!, el.getAttribute('data-hidden-comment-ids')!.split(',').filter(Boolean));
}

export function fullyExpandedGap(gap: DiffGap): DiffGap {
  return {...gap, left: 0, right: 0, leftHunk: 0, rightHunk: 0};
}

export function getDiffGapState(el: Element, gapKey: string): DiffGapState | undefined {
  return diffGaps.get(gapStoreKey(el, gapKey));
}

function updateDiffGap(el: Element, gapKey: string, changes: Partial<DiffGapState>) {
  const key = gapStoreKey(el, gapKey);
  const state = diffGaps.get(key);
  if (state) diffGaps.set(key, {...state, ...changes});
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

// a gap, and how much of it to expand: one chunk from the end an arrow points at, or the whole gap
export type DiffGapRequest = {gap: DiffGap; direction?: GapExpandEnd};

// "single" and "updown" say which arrows a gap deserves, not an end to take a chunk from
export type GapExpandEnd = 'up' | 'down';

export function gapExpandEnd(direction: string): GapExpandEnd | undefined {
  return direction === 'up' || direction === 'down' ? direction : undefined;
}

// an arrow and expanding a whole file ask the same way, one request for every gap it names
export function excerptGapsUrl(baseUrl: string, requests: DiffGapRequest[]): string {
  const url = new URL(baseUrl, window.location.href);
  for (const {gap, direction} of requests) {
    url.searchParams.append('gap', direction ? `${gapNumbers(gap)},${direction}` : gapNumbers(gap));
  }
  return url.href;
}

// the response opens with a "colgroup", which makes the parser wrap the rows in a "tbody"
export function parseTableRows(respText: string): HTMLElement[] {
  const elTemplate = document.createElement('template'); // a template can hold "tr" elements without a table around them
  elTemplate.innerHTML = respText;
  return Array.from(elTemplate.content.querySelectorAll('tr'));
}

// the gaps of a file that still hide lines, as the diff first described them
export function pendingDiffGaps(elFileBody: Element, unexpandedOnly = false): DiffGap[] {
  const gaps = [];
  for (const el of elFileBody.querySelectorAll('.code-expander-buttons[data-gap]')) {
    const gapKey = el.getAttribute('data-gap')!;
    const state = getDiffGapState(el, gapKey);
    if (!state || !gapExpandDirection(state.current)) continue;
    if (unexpandedOnly && state.expanded.length) continue;
    gaps.push(state.original);
  }
  return gaps;
}

export function diffFileHasHiddenLines(elFileBody: Element): boolean {
  return pendingDiffGaps(elFileBody).length > 0;
}

// the response says nothing about which gap a row came from, so the gap that asked marks them
function markGapRows(rows: HTMLElement[], gap: DiffGap) {
  for (const row of rows) row.setAttribute('data-expand-gap', gapNumbers(gap));
}

// the file line of each row, a conversation row taking the one of the line it follows
function gapRowLines(rows: HTMLElement[]): number[] {
  let line = 0;
  return rows.map((el) => {
    const elNum = el.querySelector('.lines-num-new');
    if (elNum) line = Number(elNum.getAttribute('data-line-num'));
    return line;
  });
}

// a gap that ends up fully expanded keeps its rows, so opening it again costs no request
export function expandGapLines(el: Element, gapKey: string, rows: HTMLElement[], current: DiffGap) {
  const state = getDiffGapState(el, gapKey);
  if (!state) return;
  markGapRows(rows, state.original);
  const unsorted = [...state.rows, ...rows]; // arrows expand from either end, in whatever order they are clicked
  const lines = gapRowLines(unsorted);
  const allRows = unsorted.map((row, i) => ({row, line: lines[i]})).sort((a, b) => a.line - b.line).map(({row}) => row);
  updateDiffGap(el, gapKey, {current, rows: allRows, expanded: gapExpandDirection(current) ? state.expanded : allRows});
}

export function reopenGap(el: Element, gapKey: string) {
  const state = getDiffGapState(el, gapKey);
  if (state?.expanded.length) updateDiffGap(el, gapKey, {current: fullyExpandedGap(state.original), rows: state.expanded});
}

// the rows come off the screen but are kept, so closing and opening a gap again is free
export function closeGap(el: Element, gapKey: string) {
  const state = getDiffGapState(el, gapKey);
  if (state) updateDiffGap(el, gapKey, {current: state.original, rows: []});
}

// the markup lives in a template on the page, so the arrows are the ones the diff itself renders
function cloneGapMarkup(selector: string): HTMLElement {
  const elTemplate = document.querySelector<HTMLTemplateElement>('#diff-gap-markup')!;
  return elTemplate.content.querySelector<HTMLElement>(selector)!.cloneNode(true) as HTMLElement;
}

function renderGapExpander(elExpander: Element, gap: DiffGap) {
  const direction = gapExpandDirection(gap);
  elExpander.setAttribute('data-expand-direction', direction);
  const stillHidden = gap.hiddenCommentIds.filter((id) => !document.querySelector(`#issuecomment-${CSS.escape(id)}`));
  elExpander.setAttribute('data-hidden-comment-ids', `,${stillHidden.join(',')},`);

  elExpander.replaceChildren();
  if (stillHidden.length) {
    const elMore = cloneGapMarkup('.code-comment-more');
    elMore.textContent = String(stillHidden.length);
    elMore.setAttribute('data-tooltip-content', `${stillHidden.length} hidden comment(s)`);
    elExpander.append(elMore);
  }
  for (const d of direction === 'updown' ? ['down', 'up'] : (direction ? [direction] : [])) {
    elExpander.append(cloneGapMarkup(`[data-gap-direction="${d}"]`));
  }
}

// a expanded line keeps its conversations in the row that follows it, so they travel together
function conversationsOf(elRow: HTMLElement): HTMLElement | null {
  const el = elRow.nextElementSibling;
  return el?.matches('tr.add-comment') ? el as HTMLElement : null;
}

function placeGapRows(elSectionRow: HTMLElement, state: DiffGapState) {
  const wanted = new Set(state.rows);
  for (const el of elSectionRow.parentElement!.querySelectorAll<HTMLElement>(`tr[data-expand-gap="${CSS.escape(gapNumbers(state.original))}"]`)) {
    if (wanted.has(el)) continue;
    conversationsOf(el)?.remove();
    el.remove();
  }
  // what is still hidden sits between the lines expanded down from above it and up from below it
  const lines = gapRowLines(state.rows);
  let elAt = elSectionRow;
  for (const [i, elRow] of state.rows.entries()) {
    const elAdded = conversationsOf(elRow); // a conversation started on an expanded line travels with it
    const elTail = elAdded && !wanted.has(elAdded) ? elAdded : null;
    if (lines[i] <= state.current.lastRight) {
      elSectionRow.before(elRow);
    } else {
      elAt.after(elRow);
      elAt = elTail ?? elRow;
    }
    if (elTail) elRow.after(elTail);
  }
}

// one effect per gap renders its arrows, hidden comment count and expanded lines from one state
export function initDiffGapExpander(el: HTMLElement) {
  const gap = parseDiffGap(el);
  const key = gapStoreKey(el, el.getAttribute('data-gap')!); // what the gap started as, not what is left of it
  diffGaps.set(key, {original: gap, current: gap, rows: [], expanded: []}); // a re-rendered file box starts over
  const scope = effectScope();
  scope.run(() => watchEffect(() => {
    if (!el.isConnected) return scope.stop(); // the file box was replaced, nothing left to render into
    const state = diffGaps.get(key);
    if (!state) return;
    renderGapExpander(el, state.current);
    const elSectionRow = el.closest<HTMLElement>('tr')!;
    placeGapRows(elSectionRow, state);
    toggleElem(elSectionRow, Boolean(gapExpandDirection(state.current)));
  }));
}
