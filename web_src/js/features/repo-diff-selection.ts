import {registerGlobalEventFunc, registerGlobalInitFunc, registerGlobalSelectorFunc} from '../modules/observer.ts';
import {selectDiffTreeFile} from '../modules/diff-file.ts';
import {addDelegatedEventListener, toggleElemClass} from '../utils/dom.ts';
import {pushLineSelectionHash, selectLineRows} from './repo-code.ts';

type DiffSide = 'L' | 'R';
export type DiffLine = `${DiffSide}${number}`;
type LineRange = {start: DiffLine; end: DiffLine};
export type DiffLineHash = LineRange & {fileHash: string};
type DiffEndpoint = {row: HTMLTableRowElement; line: DiffLine};
export type DiffCommentRange = {start: DiffEndpoint; end: DiffEndpoint};
type DiffRange = DiffCommentRange & {rows: HTMLTableRowElement[]};
type OpenComment = (range: DiffCommentRange, draft?: HTMLElement) => Promise<void>;
type SelectionAnchor = {line: DiffLine; fromGutter?: boolean};
type Gesture = {id: number; anchor: DiffEndpoint; target?: DiffEndpoint};

const lineLink = '.diff-line-link';
const visibleButton = '.add-code-comment:not(.tw-invisible)';
const selectableCell = 'td[data-diff-column]';
const savedRange = '.conversation-holder[data-start-idx]';
export const diffDraftHolder = '.diff-comment-draft';

const controllers = new WeakMap<HTMLTableElement, {
  click: (button: HTMLButtonElement, event: MouseEvent) => Promise<void> | undefined;
  repaint: () => void;
}>();
let selection: {table: HTMLTableElement; range: LineRange; anchor: SelectionAnchor} | undefined;

export function parseDiffLineHash(hash: string): DiffLineHash | undefined {
  const match = /^#diff-([0-9a-f]{40})([LR][1-9]\d*)(?:-([LR][1-9]\d*))?$/.exec(hash);
  if (!match) return;
  const start = match[2] as DiffLine;
  return {fileHash: match[1], start, end: (match[3] as DiffLine | undefined) ?? start};
}

export function formatDiffLineHash({fileHash, start, end}: DiffLineHash) {
  return `#diff-${fileHash}${start}${start === end ? '' : `-${end}`}`;
}

export function diffLineFromForm(side: string, line: string): DiffLine {
  return `${side === 'previous' ? 'L' : 'R'}${Number(line)}`;
}

function endpointFromLink(link: HTMLAnchorElement, fileHash: string): DiffEndpoint {
  return {row: link.closest('tr')!, line: link.getAttribute('href')!.slice('#diff-'.length + fileHash.length) as DiffLine};
}

export function findDiffLine(table: Element, fileHash: string, line: DiffLine) {
  return table.querySelector<HTMLAnchorElement>(`${lineLink}[href="${CSS.escape(formatDiffLineHash({fileHash, start: line, end: line}))}"]`);
}

function lineOnSide(row: HTMLTableRowElement, fileHash: string, side: DiffSide) {
  const prefix = `#diff-${fileHash}${side}`;
  const link = row.querySelector<HTMLAnchorElement>(`:scope > td > ${lineLink}[href^="${CSS.escape(prefix)}"]`);
  return link ? endpointFromLink(link, fileHash) : undefined;
}

function rangeBetween(first: DiffEndpoint, last: DiffEndpoint): DiffRange {
  if (first.row === last.row ? first.line[0] > last.line[0] :
    first.row.compareDocumentPosition(last.row) & Node.DOCUMENT_POSITION_PRECEDING) {
    [first, last] = [last, first];
  }
  const rows = selectLineRows(first.row, last.row).filter((row) => !row.matches('.tag-code, .add-comment'));
  return {start: first, end: last, rows};
}

export function findDiffLineExpander(table: Element, line: DiffLine) {
  const side = line[0] === 'L' ? 'left' : 'right';
  const number = Number(line.slice(1));
  let closest: HTMLElement | undefined;
  let distance = Infinity;
  for (const button of table.querySelectorAll<HTMLElement>('.code-expander-button')) {
    const {searchParams: params} = new URL(button.getAttribute('data-fetch-url')!, window.location.href);
    const last = Number(params.get(`last_${side}`));
    const first = Number(params.get(side));
    const isTail = params.get('left_hunk_size') === '0' && params.get('right_hunk_size') === '0';
    if (number <= last || number > first || (number === first && !isTail)) continue;
    const direction = params.get('direction');
    if (direction === 'single') return button;
    const distanceToEdge = direction === 'down' ? number - last : first - number;
    if (distanceToEdge < distance) {
      closest = button;
      distance = distanceToEdge;
    }
  }
  return closest;
}

export function resolveDiffRange(table: Element, {fileHash, start, end}: DiffLineHash, allowHiddenStart = false): DiffRange | undefined {
  const first = findDiffLine(table, fileHash, start);
  const last = end === start ? first : findDiffLine(table, fileHash, end);
  if (!last) return;
  const endPoint = endpointFromLink(last, fileHash);
  if (first) return rangeBetween(endpointFromLink(first, fileHash), endPoint);
  const gap = allowHiddenStart ? findDiffLineExpander(table, start)?.closest('tr') : undefined;
  if (gap && gap.compareDocumentPosition(endPoint.row) & Node.DOCUMENT_POSITION_FOLLOWING) {
    return rangeBetween({row: gap, line: start}, endPoint);
  }
  return undefined;
}

export function normalizeDiffCommentRange({start, end}: DiffCommentRange, clicked: DiffEndpoint, fileHash: string): DiffCommentRange {
  if (start.row === end.row && (start.line === end.line || end.row.getAttribute('data-line-type') === 'same')) {
    return {start: clicked, end: clicked};
  }
  if (start.line[0] === 'R' && end.line[0] === 'L') {
    if (start.row.matches('.same-code')) return {start: lineOnSide(start.row, fileHash, 'L')!, end};
    if (end.row.matches('.same-code')) return {start, end: lineOnSide(end.row, fileHash, 'R')!};
    return {start: clicked, end: clicked};
  }
  return {start, end};
}

export function setDiffLineSelection(table?: HTMLTableElement, range?: DiffCommentRange, anchor?: SelectionAnchor) {
  const previous = selection?.table;
  selection = table && range ? {
    table, range: {start: range.start.line, end: range.end.line}, anchor: anchor ?? {line: range.start.line, fromGutter: true},
  } : undefined;
  if (previous && previous !== table) repaintDiffRanges(previous);
  if (table) repaintDiffRanges(table);
  document.querySelector('.diff-hash-target')?.classList.remove('diff-hash-target');
}

function navigateDiffLineSelection(table: HTMLTableElement, fileHash: string, range: DiffCommentRange, anchor: SelectionAnchor = {line: range.start.line}) {
  setDiffLineSelection(table, range, anchor);
  selectDiffTreeFile(fileHash);
  pushLineSelectionHash(formatDiffLineHash({fileHash, start: range.start.line, end: range.end.line}));
}

export function repaintDiffRanges(element: Element) {
  const table = element.closest('table');
  if (table) controllers.get(table)?.repaint();
}

export function setAddCommentVisibility(row: Element, side: string, visible: boolean) {
  const selector = row.getAttribute('data-line-type') === 'same' ? '.add-code-comment' :
    `.add-code-comment[data-side="${CSS.escape(side)}"]`;
  toggleElemClass(row.querySelectorAll(selector), 'tw-invisible', !visible);
}

function diffTableHash(table: HTMLTableElement) {
  return table.closest('.diff-file-box')!.id.slice('diff-'.length);
}

function initDiffSelection(table: HTMLTableElement, openComment: OpenComment) {
  const fileHash = diffTableHash(table);
  let pointer: Gesture | undefined;
  let clicked: {endpoint: DiffEndpoint; opened: Promise<void>; draft: HTMLElement} | undefined;
  let suppressClick = false;
  let painted = new Set<HTMLTableRowElement>();
  let hovered: Element | null = null, focused: Element | null = null;

  function repaint() {
    if (clicked && !clicked.draft.isConnected) clicked = undefined;
    const next = new Set<HTMLTableRowElement>();
    function paint(start: DiffLine, end: DiffLine, allowHiddenStart = false) {
      for (const row of resolveDiffRange(table, {fileHash, start, end}, allowHiddenStart)?.rows ?? []) next.add(row);
    }

    if (pointer?.target) {
      for (const row of rangeBetween(pointer.anchor, pointer.target).rows) next.add(row);
    } else if (selection?.table === table) {
      paint(selection.range.start, selection.range.end);
    }
    for (const form of table.querySelectorAll<HTMLFormElement>(`${diffDraftHolder} form`)) {
      const value = (name: string) => (form.elements.namedItem(name) as HTMLInputElement).value;
      if (value('start_line')) paint(diffLineFromForm(value('start_side'), value('start_line')), diffLineFromForm(value('side'), value('line')));
    }
    for (const holder of new Set([hovered, focused])) {
      if (!holder || !table.contains(holder)) continue;
      const signedLine = (name: string): DiffLine => {
        const line = Number(holder.getAttribute(name));
        return `${line < 0 ? 'L' : 'R'}${Math.abs(line)}`;
      };
      paint(signedLine('data-start-idx'), signedLine('data-idx'), true);
    }
    for (const row of painted) {
      if (!next.has(row)) row.classList.remove('diff-range-highlight');
    }
    for (const row of next) {
      if (!painted.has(row)) row.classList.add('diff-range-highlight');
    }
    painted = next;
  }

  function commentEndpoint(button: Element) {
    return lineOnSide(button.closest('tr')!, fileHash, button.getAttribute('data-side') === 'left' ? 'L' : 'R')!;
  }

  function canComment({row, line}: DiffEndpoint) {
    const side = line[0] === 'L' ? 'left' : 'right';
    return row.querySelector(`:scope > td > ${visibleButton}[data-side="${CSS.escape(side)}"]`) !== null;
  }

  function commentRange({start, end}: DiffRange, target: DiffEndpoint) {
    if (start.row !== end.row && end.row.matches('.same-code') && !canComment(end)) {
      end = lineOnSide(end.row, fileHash, end.line[0] === 'L' ? 'R' : 'L')!;
    }
    const normalized = normalizeDiffCommentRange({start, end}, target, fileHash);
    return canComment(normalized.end) ? normalized : {start: target, end: target};
  }

  async function openRange(range: DiffCommentRange, anchor?: typeof clicked) {
    if (anchor) await anchor.opened;
    await openComment(range, anchor?.draft.isConnected ? anchor.draft : undefined);
  }

  function selectCommentRange(first: DiffEndpoint, last: DiffEndpoint, previous?: typeof clicked) {
    const range = commentRange(rangeBetween(first, last), last);
    navigateDiffLineSelection(table, fileHash, range, {line: range.start.row === first.row ? range.start.line : range.end.line});
    return openRange(range, previous);
  }

  function takeClicked() {
    const previous = clicked;
    clicked = undefined;
    return previous?.draft.isConnected ? previous : undefined;
  }

  for (const event of ['pointerover', 'pointerout', 'focusin', 'focusout']) {
    addDelegatedEventListener<HTMLElement, PointerEvent | FocusEvent>(table, event, savedRange, (holder, e) => {
      const related = (e.relatedTarget as Element | null)?.closest(savedRange) ?? null;
      if (related === holder) return;
      const active = event === 'pointerout' || event === 'focusout' ? related : holder;
      if (e instanceof FocusEvent) focused = active;
      else hovered = active;
      repaint();
    });
  }

  table.addEventListener('pointerdown', (e) => {
    if (e.button !== 0 || !e.isPrimary) return;
    suppressClick = false;
    const button = (e.target as Element).closest<HTMLButtonElement>(visibleButton);
    if (!button || e.shiftKey) return;
    e.preventDefault();
    button.setPointerCapture(e.pointerId);
    const anchor = commentEndpoint(button);
    pointer = {id: e.pointerId, anchor, target: anchor};
  });

  function pointerTarget(e: PointerEvent) {
    const cell = table.ownerDocument.elementFromPoint(e.clientX, e.clientY)?.closest(selectableCell);
    if (!cell || cell.closest('table') !== table) return;
    const column = cell.getAttribute('data-diff-column')!;
    const button = cell.parentElement!.querySelector(`:scope > [data-diff-column="${CSS.escape(column)}"] > .add-code-comment`);
    return button ? commentEndpoint(button) : undefined;
  }

  table.addEventListener('pointermove', (e) => {
    if (pointer?.id !== e.pointerId) return;
    const target = pointerTarget(e);
    if (target?.line === pointer.target?.line) return;
    pointer.target = target;
    repaint();
  });

  table.addEventListener('pointerup', (e) => {
    if (pointer?.id !== e.pointerId) return;
    const {anchor} = pointer;
    const target = pointerTarget(e);
    pointer = undefined;
    suppressClick = target?.line !== anchor.line;
    if (target && suppressClick) {
      const previous = takeClicked();
      selectCommentRange(anchor, target, previous?.endpoint.line === anchor.line ? previous : undefined);
    } else {
      repaint();
    }
  });

  for (const event of ['pointercancel', 'lostpointercapture']) {
    table.addEventListener(event, (e) => {
      if (pointer?.id !== (e as PointerEvent).pointerId) return;
      pointer = undefined;
      repaint();
    });
  }

  controllers.set(table, {repaint, click(button, e) {
    if (suppressClick && e.detail > 0) return;
    const endpoint = commentEndpoint(button);
    const previous = takeClicked();
    const activeSelection = selection?.table === table ? selection : undefined;
    if (e.shiftKey) {
      const anchor = activeSelection?.anchor.fromGutter ?
        endpointFromLink(findDiffLine(table, fileHash, activeSelection.anchor.line)!, fileHash) : previous?.endpoint;
      if (anchor) {
        const first = anchor.row === endpoint.row && endpoint.line[0] === 'L' ? endpoint : anchor;
        return selectCommentRange(first, endpoint, previous?.endpoint.line === anchor.line ? previous : undefined);
      }
    }
    const single = {start: endpoint, end: endpoint};
    if (activeSelection) {
      const range = resolveDiffRange(table, {...activeSelection.range, fileHash})!;
      if (range.end.row === endpoint.row) {
        if (range.end.line === endpoint.line || endpoint.row.matches('.same-code')) {
          const comment = commentRange(range, endpoint);
          if (comment.start.line !== comment.end.line) return openRange(comment);
        }
        if (range.start.line !== endpoint.line || range.end.line !== endpoint.line) navigateDiffLineSelection(table, fileHash, single);
      }
    }
    const opened = openComment(single);
    const side = endpoint.line[0] === 'L' ? 'left' : 'right';
    const draft = endpoint.row.nextElementSibling?.querySelector<HTMLElement>(`:scope > .add-comment-${side} > ${diffDraftHolder}`);
    if (draft) clicked = {endpoint, opened, draft};
    return opened;
  }});
}

export function registerRepoDiffSelection(openComment: OpenComment) {
  registerGlobalInitFunc('initDiffSelection', (table: HTMLTableElement) => initDiffSelection(table, openComment));
  registerGlobalInitFunc('repaintDiffRanges', repaintDiffRanges);
  registerGlobalSelectorFunc<HTMLAnchorElement>(`${lineLink}:not([tabindex])`, (link) => { link.tabIndex = -1 });
  registerGlobalEventFunc('click', 'onSelectDiffLine', (link: HTMLAnchorElement, e: MouseEvent) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    e.preventDefault();
    const table = link.closest('table')!;
    const fileHash = diffTableHash(table);
    const end = endpointFromLink(link, fileHash);
    const anchor = e.shiftKey && selection?.table === table ? selection.anchor.line : end.line;
    const range = resolveDiffRange(table, {fileHash, start: anchor, end: end.line})!;
    navigateDiffLineSelection(table, fileHash, range, {line: anchor, fromGutter: true});
    window.getSelection()!.removeAllRanges();
  });
  registerGlobalEventFunc('click', 'onAddDiffComment', (button: HTMLButtonElement, e: MouseEvent) => {
    e.preventDefault();
    if (!button.matches(visibleButton)) return;
    return controllers.get(button.closest('table')!)!.click(button, e);
  });
}
