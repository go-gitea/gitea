import {addDelegatedEventListener} from '../utils/dom.ts';
import {sleep} from '../utils.ts';
import {setFileFolding} from './file-fold.ts';

const diffLineNumberCellSelector = '#diff-file-boxes .code-diff td.lines-num[data-line-num]';
const diffAnchorSuffixRegex = /([LR])(\d+)$/;
const diffHashRangeRegex = /^(diff-[0-9a-f]+)([LR]\d+)(?:-([LR]\d+))?$/i;

type DiffAnchorSide = 'L' | 'R';
type DiffAnchorInfo = {anchor: string, fragment: string, side: DiffAnchorSide, line: number};
type DiffSelectionState = DiffAnchorInfo & {container: HTMLElement};
type DiffSelectionRange = {fragment: string, startSide: DiffAnchorSide, startLine: number, endSide: DiffAnchorSide, endLine: number};

let diffSelectionStart: DiffSelectionState | null = null;

function changeHash(hash: string) {
  if (window.history.pushState) {
    window.history.pushState(null, null, hash);
  } else {
    window.location.hash = hash;
  }
}

function parseDiffAnchor(anchor: string | null): DiffAnchorInfo | null {
  if (!anchor || !anchor.startsWith('diff-')) return null;
  const suffixMatch = diffAnchorSuffixRegex.exec(anchor);
  if (!suffixMatch) return null;
  const line = Number.parseInt(suffixMatch[2]);
  if (Number.isNaN(line)) return null;
  const fragment = anchor.slice(0, -suffixMatch[0].length);
  const side = suffixMatch[1] as DiffAnchorSide;
  return {anchor, fragment, side, line};
}

function applyDiffLineSelection(container: HTMLElement, range: DiffSelectionRange, options?: {updateHash?: boolean}): boolean {
  // Find the start and end anchor elements
  const startId = `${range.fragment}${range.startSide}${range.startLine}`;
  const endId = `${range.fragment}${range.endSide}${range.endLine}`;
  const startSpan = container.querySelector<HTMLElement>(`#${CSS.escape(startId)}`);
  const endSpan = container.querySelector<HTMLElement>(`#${CSS.escape(endId)}`);

  if (!startSpan || !endSpan) return false;

  const startTr = startSpan.closest('tr');
  const endTr = endSpan.closest('tr');
  if (!startTr || !endTr) return false;

  // Clear previous selection
  for (const tr of document.querySelectorAll('.code-diff tr.active')) {
    tr.classList.remove('active');
  }

  // Get all rows in the diff section
  const allRows = Array.from(container.querySelectorAll<HTMLElement>('.code-diff tbody tr'));
  const startIndex = allRows.indexOf(startTr);
  const endIndex = allRows.indexOf(endTr);

  if (startIndex === -1 || endIndex === -1) return false;

  // Select all rows between start and end (inclusive)
  const minIndex = Math.min(startIndex, endIndex);
  const maxIndex = Math.max(startIndex, endIndex);

  for (let i = minIndex; i <= maxIndex; i++) {
    const row = allRows[i];
    // Only select rows that are actual diff lines (not comment rows, expansion buttons, etc.)
    // Skip rows with data-line-type="4" which are code expansion buttons
    if (row.querySelector('td.lines-num') && row.getAttribute('data-line-type') !== '4') {
      row.classList.add('active');
    }
  }

  if (options?.updateHash !== false) {
    const startAnchor = `${range.fragment}${range.startSide}${range.startLine}`;
    const hashValue = (range.startSide === range.endSide && range.startLine === range.endLine) ?
      startAnchor :
      `${startAnchor}-${range.endSide}${range.endLine}`;
    changeHash(`#${hashValue}`);
  }
  return true;
}

export function parseDiffHashRange(hashValue: string): DiffSelectionRange | null {
  if (!hashValue.startsWith('diff-')) return null;
  const match = diffHashRangeRegex.exec(hashValue);
  if (!match) return null;
  const startInfo = parseDiffAnchor(`${match[1]}${match[2]}`);
  if (!startInfo) return null;
  let endSide = startInfo.side;
  let endLine = startInfo.line;
  if (match[3]) {
    const endInfo = parseDiffAnchor(`${match[1]}${match[3]}`);
    if (!endInfo) {
      return {fragment: startInfo.fragment, startSide: startInfo.side, startLine: startInfo.line, endSide: startInfo.side, endLine: startInfo.line};
    }
    endSide = endInfo.side;
    endLine = endInfo.line;
  }
  return {
    fragment: startInfo.fragment,
    startSide: startInfo.side,
    startLine: startInfo.line,
    endSide,
    endLine,
  };
}

export async function highlightDiffSelectionFromHash(): Promise<boolean> {
  const {hash} = window.location;
  if (!hash || !hash.startsWith('#diff-')) return false;
  const range = parseDiffHashRange(hash.substring(1));
  if (!range) return false;
  const targetId = `${range.fragment}${range.startSide}${range.startLine}`;

  // Wait for the target element to be available (in case it needs to be loaded)
  const targetSpan = document.querySelector<HTMLElement>(`#${CSS.escape(targetId)}`);
  if (!targetSpan) {
    // Target not found - it might need to be loaded via "show more files"
    // Return false to let onLocationHashChange handle the loading
    return false;
  }

  const container = targetSpan.closest<HTMLElement>('.diff-file-box');
  if (!container) return false;

  // Check if the file is collapsed and expand it if needed
  if (container.getAttribute('data-folded') === 'true') {
    const foldBtn = container.querySelector<HTMLElement>('.fold-file');
    if (foldBtn) {
      // Expand the file using the setFileFolding utility
      setFileFolding(container, foldBtn, false);
      // Wait a bit for the expansion animation
      await sleep(100);
    }
  }

  if (!applyDiffLineSelection(container, range, {updateHash: false})) return false;
  diffSelectionStart = {
    anchor: targetId,
    fragment: range.fragment,
    side: range.startSide,
    line: range.startLine,
    container,
  };

  // Scroll to the first selected line (scroll to the tr element, not the span)
  // The span is an inline element inside td, we need to scroll to the tr for better visibility
  await sleep(10);
  const targetTr = targetSpan.closest('tr');
  if (targetTr) {
    targetTr.scrollIntoView({behavior: 'smooth', block: 'center'});
  }
  return true;
}

function handleDiffLineNumberClick(cell: HTMLElement, e: MouseEvent) {
  let span = cell.querySelector<HTMLSpanElement>('span[id^="diff-"]');
  let info = parseDiffAnchor(span?.id ?? null);

  // If clicked cell has no line number (e.g., clicking on the empty side of a deletion/addition),
  // try to find the line number from the sibling cell on the same row
  if (!info) {
    const row = cell.closest('tr');
    if (!row) return;
    // Find the other line number cell in the same row
    const siblingCell = cell.classList.contains('lines-num-old') ?
      row.querySelector<HTMLElement>('td.lines-num-new') :
      row.querySelector<HTMLElement>('td.lines-num-old');
    if (siblingCell) {
      span = siblingCell.querySelector<HTMLSpanElement>('span[id^="diff-"]');
      info = parseDiffAnchor(span?.id ?? null);
    }
    if (!info) return;
  }

  const container = cell.closest<HTMLElement>('.diff-file-box');
  if (!container) return;

  e.preventDefault();

  // Check if clicking on a single already-selected line without shift key - deselect it
  if (!e.shiftKey) {
    const clickedRow = cell.closest('tr');
    if (clickedRow?.classList.contains('active')) {
      // Check if this is a single-line selection by checking if it's the only selected line
      const selectedRows = container.querySelectorAll('.code-diff tr.active');
      if (selectedRows.length === 1) {
        // This is a single selected line, deselect it
        clickedRow.classList.remove('active');
        diffSelectionStart = null;
        // Remove hash from URL completely
        if (window.history.pushState) {
          window.history.pushState(null, null, window.location.pathname + window.location.search);
        } else {
          window.location.hash = '';
        }
        window.getSelection().removeAllRanges();
        return;
      }
    }
  }

  let rangeStart: DiffAnchorInfo = info;
  if (e.shiftKey && diffSelectionStart &&
    diffSelectionStart.container === container &&
    diffSelectionStart.fragment === info.fragment) {
    rangeStart = diffSelectionStart;
  }

  const range: DiffSelectionRange = {
    fragment: info.fragment,
    startSide: rangeStart.side,
    startLine: rangeStart.line,
    endSide: info.side,
    endLine: info.line,
  };

  if (applyDiffLineSelection(container, range)) {
    diffSelectionStart = {...info, container};
    window.getSelection().removeAllRanges();
  }
}

export function initDiffLineSelection() {
  addDelegatedEventListener<HTMLElement, MouseEvent>(document, 'click', diffLineNumberCellSelector, (cell, e) => {
    if (e.defaultPrevented) return;
    // Ignore clicks on or inside code-expander-buttons
    const target = e.target as HTMLElement;
    if (target.closest('.code-expander-button') || target.closest('.code-expander-buttons') ||
      target.closest('button, a, input, select, textarea, summary, [role="button"]')) {
      return;
    }
    handleDiffLineNumberClick(cell, e);
  });
  window.addEventListener('hashchange', () => {
    highlightDiffSelectionFromHash();
  });
  highlightDiffSelectionFromHash();
}
