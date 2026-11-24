import {registerGlobalInitFunc, registerGlobalSelectorFunc} from '../modules/observer.ts';
import {addDelegatedEventListener} from '../utils/dom.ts';

type DiffLine = `${'L' | 'R'}${number}`;
export type DiffLineHash = {fileHash: string; start: DiffLine; end: DiffLine};
type Endpoint = {row: HTMLTableRowElement; line: DiffLine};
export type DiffCommentRange = {start: Endpoint; end: Endpoint};

const lineLink = '.diff-line-link';
const commentButton = '.add-code-comment:not(.tw-invisible)';
const savedRange = '.conversation-holder[data-range-start]';
export const diffDraftHolder = '.diff-comment-draft';

const repainters = new WeakMap<HTMLTableElement, () => void>();
let selection: {table: HTMLTableElement; range: DiffCommentRange; anchor: Endpoint; hash: string} | undefined;

export function parseDiffLineHash(hash: string): DiffLineHash | undefined {
  const match = /^#diff-([0-9a-f]{40})([LR][1-9]\d*)(?:-([LR][1-9]\d*))?$/.exec(hash);
  if (!match) return;
  const start = match[2] as DiffLine;
  return {fileHash: match[1], start, end: (match[3] as DiffLine | undefined) ?? start};
}

function toEndpoint(link: HTMLAnchorElement): Endpoint {
  return {row: link.closest('tr')!, line: parseDiffLineHash(link.hash)!.start};
}

function orderRange(start: Endpoint, end: Endpoint): DiffCommentRange {
  if (start.row === end.row ? start.line[0] > end.line[0] :
    start.row.compareDocumentPosition(end.row) & Node.DOCUMENT_POSITION_PRECEDING) {
    return {start: end, end: start};
  }
  return {start, end};
}

export function findDiffLine(table: Element, fileHash: string, line: DiffLine) {
  return table.querySelector<HTMLAnchorElement>(`${lineLink}[href="${CSS.escape(`#diff-${fileHash}${line}`)}"]`);
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

export function resolveDiffRange(table: Element, {fileHash, start, end}: DiffLineHash, allowHiddenStart = false): DiffCommentRange | undefined {
  const first = findDiffLine(table, fileHash, start);
  const last = findDiffLine(table, fileHash, end);
  if (!last) return;
  const target = toEndpoint(last);
  if (first) return orderRange(toEndpoint(first), target);
  const gap = allowHiddenStart ? findDiffLineExpander(table, start)?.closest('tr') : undefined;
  return gap && gap.compareDocumentPosition(target.row) & Node.DOCUMENT_POSITION_FOLLOWING ?
    {start: {row: gap, line: start}, end: target} : undefined;
}

export function isDiffLineSelected(hash: string) {
  return selection?.hash === hash && selection.table.isConnected;
}

export function setDiffLineSelection(table?: HTMLTableElement, range?: DiffCommentRange, anchor = range?.start) {
  const previous = selection?.table;
  if (table && range) {
    const {start, end} = range;
    const fileId = table.closest('.diff-file-box')!.id;
    selection = {table, range, anchor: anchor!, hash: `#${fileId}${start.line}${start.line === end.line ? '' : `-${end.line}`}`};
  } else {
    selection = undefined;
  }
  if (previous && previous !== table) repaintDiffRanges(previous);
  if (table) repaintDiffRanges(table);
}

export function repaintDiffRanges(element: Element) {
  const table = element.closest('table');
  if (table) repainters.get(table)?.();
}

function sideLine(side: string, line: string): DiffLine {
  return `${side === 'previous' || side === 'left' ? 'L' : 'R'}${Number(line)}`;
}

export function registerRepoDiffSelection(openComment: (range: DiffCommentRange, draft?: HTMLElement) => Promise<void>) {
  registerGlobalSelectorFunc('table[data-new-comment-url] > tbody > tr > .lines-num > span[rel^="diff-"]', (marker) => {
    if (marker.nextElementSibling?.matches(lineLink)) return;
    const link = document.createElement('a');
    link.className = 'diff-line-link tw-absolute tw-inset-0';
    link.href = `#${marker.getAttribute('rel')!}`;
    link.setAttribute('aria-label', parseDiffLineHash(link.hash)!.start);
    marker.parentElement!.classList.add('tw-relative');
    marker.after(link);
  });
  registerGlobalInitFunc('initDiffSelection', (table: HTMLTableElement) => {
    const fileHash = table.closest('.diff-file-box')!.id.slice('diff-'.length);
    const split = table.closest('.code-diff-split') !== null;
    let hovered: Element | null = null;
    let focused: Element | null = null;
    let drag: {id: number; start: Endpoint; end?: Endpoint} | undefined;
    let suppressClick = false;
    let clicked: {endpoint: Endpoint; draft: HTMLElement; opened: Promise<void>} | undefined;

    function repaint() {
      const rows = new Set<Element>();
      function paint(range?: DiffCommentRange) {
        if (!range) return;
        for (let row: Element | null = range.start.row; row; row = row.nextElementSibling) {
          if (!row.matches('.tag-code, .add-comment')) rows.add(row);
          if (row === range.end.row) break;
        }
      }
      if (drag?.end) paint(orderRange(drag.start, drag.end));
      else if (selection?.table === table) paint(selection.range);

      for (const form of table.querySelectorAll<HTMLFormElement>(`${diffDraftHolder} form`)) {
        const field = (name: string) => (form.elements.namedItem(name) as HTMLInputElement).value;
        if (field('start_line')) paint(resolveDiffRange(table, {
          fileHash, start: sideLine(field('start_side'), field('start_line')), end: sideLine(field('side'), field('line')),
        }));
      }
      for (const holder of new Set([hovered, focused])) {
        if (!holder || !table.contains(holder)) continue;
        const line = (name: string): DiffLine => {
          const number = Number(holder.getAttribute(name));
          return `${number < 0 ? 'L' : 'R'}${Math.abs(number)}`;
        };
        paint(resolveDiffRange(table, {fileHash, start: line('data-range-start'), end: line('data-range-end')}, true));
      }
      for (const row of table.querySelectorAll(':scope > tbody > .diff-range-highlight')) {
        if (!rows.has(row)) row.classList.remove('diff-range-highlight');
      }
      for (const row of rows) row.classList.add('diff-range-highlight');
    }

    repainters.set(table, repaint);
    new MutationObserver(repaint).observe(table.tBodies[0], {childList: true});

    function select(range: DiffCommentRange, anchor: Endpoint) {
      setDiffLineSelection(table, range, anchor);
      window.location.hash = selection!.hash;
    }

    function buttonEndpoint(button: HTMLElement): Endpoint {
      return {row: button.closest('tr')!, line: sideLine(button.getAttribute('data-side')!, button.getAttribute('data-idx')!)};
    }

    function canComment({row, line}: Endpoint) {
      const side = line[0] === 'L' ? 'left' : 'right';
      return row.querySelector(`${commentButton}[data-side="${CSS.escape(side)}"]`) !== null;
    }

    function onSide(endpoint: Endpoint, side: 'old' | 'new') {
      return toEndpoint(endpoint.row.querySelector<HTMLAnchorElement>(`.lines-num-${CSS.escape(side)} > ${lineLink}`)!);
    }

    function commentRange({start, end}: DiffCommentRange, target: Endpoint) {
      const single = {start: target, end: target};
      if (start.row === end.row && (start.line === end.line || end.row.matches('.same-code'))) return single;
      if (end.row.matches('.same-code') && !canComment(end)) {
        end = onSide(end, end.line[0] === 'L' ? 'new' : 'old');
      }
      if (start.line[0] === 'R' && end.line[0] === 'L') {
        if (start.row.matches('.same-code')) {
          start = onSide(start, 'old');
        } else if (end.row.matches('.same-code')) {
          end = onSide(end, 'new');
        } else {
          return single;
        }
      }
      return canComment(end) ? {start, end} : single;
    }

    async function extendComment(start: Endpoint, end: Endpoint) {
      const range = commentRange(orderRange(start, end), end);
      select(range, range.start.row === start.row ? range.start : range.end);
      const previous = clicked?.endpoint.line === start.line ? clicked : undefined;
      clicked = undefined;
      if (previous) await previous.opened;
      return openComment(range, previous?.draft.isConnected ? previous.draft : undefined);
    }

    addDelegatedEventListener<HTMLAnchorElement, MouseEvent>(table, 'click', lineLink, (link, e) => {
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      e.preventDefault();
      const end = toEndpoint(link);
      const start = e.shiftKey && selection?.table === table ? selection.anchor : end;
      clicked = undefined;
      select(orderRange(start, end), start);
      window.getSelection()!.removeAllRanges();
    });

    addDelegatedEventListener<HTMLButtonElement, MouseEvent>(table, 'click', commentButton, (button, e) => {
      e.preventDefault();
      if (suppressClick && e.detail > 0) return;
      if (clicked && !clicked.draft.isConnected) clicked = undefined;
      const endpoint = buttonEndpoint(button);
      const active = selection?.table === table ? selection : undefined;
      const anchor = active?.anchor ?? clicked?.endpoint;
      if (e.shiftKey && anchor) return extendComment(anchor, endpoint);

      if (active?.range.end.row === endpoint.row) {
        if (active.range.end.line === endpoint.line || endpoint.row.matches('.same-code')) {
          const range = commentRange(active.range, endpoint);
          if (range.start.line !== range.end.line) return openComment(range);
        }
        if (active.range.start.line !== endpoint.line || active.range.end.line !== endpoint.line) {
          select({start: endpoint, end: endpoint}, endpoint);
        }
      }
      const opened = openComment({start: endpoint, end: endpoint});
      const draft = endpoint.row.nextElementSibling?.querySelector<HTMLElement>(`.add-comment-${button.getAttribute('data-side')!} > ${diffDraftHolder}`);
      clicked = draft ? {endpoint, opened, draft} : undefined;
      return opened;
    });

    function pointerTarget(e: PointerEvent): Endpoint | undefined {
      const cell = document.elementFromPoint(e.clientX, e.clientY)?.closest('td');
      if (cell?.parentElement?.parentElement !== table.tBodies[0] || cell.parentElement.matches('.tag-code')) return;
      const column = split ? (cell.cellIndex < 4 ? '.lines-code-old' : '.lines-code-new') : '.lines-code';
      const button = cell.parentElement.querySelector<HTMLElement>(`${column} > .add-code-comment`);
      return button ? buttonEndpoint(button) : undefined;
    }

    table.addEventListener('pointerdown', (e) => {
      if (e.button !== 0 || !e.isPrimary) return;
      suppressClick = false;
      const button = (e.target as Element).closest<HTMLElement>(commentButton);
      if (!button || e.shiftKey) return;
      e.preventDefault();
      button.setPointerCapture(e.pointerId);
      drag = {id: e.pointerId, start: buttonEndpoint(button)};
    });

    table.addEventListener('pointermove', (e) => {
      if (drag?.id !== e.pointerId) return;
      const end = pointerTarget(e);
      if (end?.line === drag.end?.line) return;
      drag.end = end;
      repaint();
    });

    for (const event of ['pointerup', 'pointercancel', 'lostpointercapture'] as const) {
      table.addEventListener(event, (e) => {
        if (drag?.id !== e.pointerId) return;
        const {start} = drag;
        drag = undefined;
        const end = event === 'pointerup' ? pointerTarget(e) : undefined;
        if (event === 'pointerup') suppressClick = end?.line !== start.line;
        if (end && suppressClick) extendComment(start, end);
        else repaint();
      });
    }

    for (const event of ['pointerover', 'pointerout', 'focusin', 'focusout']) {
      addDelegatedEventListener<HTMLElement, PointerEvent | FocusEvent>(table, event, savedRange, (holder, e) => {
        const related = e.relatedTarget instanceof Element ? e.relatedTarget.closest(savedRange) : null;
        if (related === holder) return;
        const active = event === 'pointerout' || event === 'focusout' ? related : holder;
        if (e instanceof FocusEvent) focused = active;
        else hovered = active;
        repaint();
      });
    }
  });
}
