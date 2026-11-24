import {registerGlobalEventFunc, registerGlobalInitFunc, registerGlobalSelectorFunc} from '../modules/observer.ts';
import {performFetchActionRequest} from '../modules/fetch-action.ts';
import {trString} from '../modules/i18n.ts';
import {showErrorToast} from '../modules/toast.ts';
import {
  addDelegatedEventListener, createElementFromAttrs, createElementFromHTML, showElem, toggleElem,
} from '../utils/dom.ts';
import {html, htmlRaw} from '../utils/html.ts';
import {getComboMarkdownEditor, initComboMarkdownEditor, toggleExtraToolbarButton} from './comp/ComboMarkdownEditor.ts';

type DiffLine = `${'L' | 'R'}${number}`;
export type DiffLineHash = {fileHash: string; start: DiffLine; end: DiffLine};
type Endpoint = {row: HTMLTableRowElement; line: DiffLine};
type DiffCommentRange = {start: Endpoint; end: Endpoint};

const lineLink = '.diff-line-link';
const commentButton = '.add-code-comment';

const repainters = new WeakMap<HTMLTableElement, () => void>();
let selection: {table: HTMLTableElement; range: DiffCommentRange; anchor: Endpoint; hash: string} | undefined;

export function parseDiffLineHash(hash: string): DiffLineHash | undefined {
  const match = /^#diff-([0-9a-f]{40})([LR][1-9]\d*)(?:-([LR][1-9]\d*))?$/.exec(hash);
  if (!match) return;
  const start = match[2] as DiffLine;
  return {fileHash: match[1], start, end: (match[3] as DiffLine | undefined) ?? start};
}

function toEndpoint(link: HTMLAnchorElement): Endpoint {
  return {row: link.closest<HTMLTableRowElement>('.diff-review-line')!, line: parseDiffLineHash(link.hash)!.start};
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
  for (const gap of table.querySelectorAll('.code-expander-buttons')) {
    const buttons = gap.querySelectorAll<HTMLElement>('.code-expander-button');
    if (!buttons.length) continue;
    const {searchParams: params} = new URL(buttons[0].getAttribute('data-fetch-url')!, window.location.href);
    const last = Number(params.get(`last_${side}`));
    const first = Number(params.get(side));
    const isTail = params.get('left_hunk_size') === '0' && params.get('right_hunk_size') === '0';
    if (number <= last || number > first || (number === first && !isTail)) continue;
    return buttons.length === 1 || number - last <= first - number ? buttons[0] : buttons[1];
  }
  return undefined;
}

export function resolveDiffRange(table: Element, {fileHash, start, end}: DiffLineHash): DiffCommentRange | undefined {
  const first = findDiffLine(table, fileHash, start);
  const last = findDiffLine(table, fileHash, end);
  if (!first || !last) return;
  return orderRange(toEndpoint(first), toEndpoint(last));
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
  if (previous && previous !== table) repainters.get(previous)?.();
  if (table) repainters.get(table)?.();
}

function signedDiffLine(number: number): DiffLine {
  return `${number < 0 ? 'L' : 'R'}${Math.abs(number)}`;
}

function sideLine(side: string, line: string): DiffLine {
  return `${side === 'previous' || side === 'left' ? 'L' : 'R'}${Number(line)}`;
}

function initDiffSelection(table: HTMLTableElement) {
  const fileHash = table.closest('.diff-file-box')!.id.slice('diff-'.length);
  const split = table.closest('.code-diff-split') !== null;
  const body = table.tBodies[0];
  let hovered: Element | null = null;
  let focused: Element | null = null;
  let drag: {id: number; start: Endpoint; end?: Endpoint} | undefined;
  let suppressClick = false;

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

    for (const form of table.querySelectorAll<HTMLFormElement>('.diff-comment-draft form')) {
      const field = (name: string) => (form.elements.namedItem(name) as HTMLInputElement).value;
      const start = Number(field('start_line'));
      if (start) paint(resolveDiffRange(table, {
        fileHash, start: signedDiffLine(start), end: sideLine(field('side'), field('line')),
      }));
    }
    for (const holder of new Set([hovered, focused])) {
      if (!holder || !table.contains(holder)) continue;
      paint(resolveDiffRange(table, {
        fileHash, start: signedDiffLine(Number(holder.getAttribute('data-range-start'))),
        end: signedDiffLine(Number(holder.getAttribute('data-range-end'))),
      }));
    }
    for (const row of body.querySelectorAll('.diff-range-highlight')) {
      if (row.parentElement === body && !rows.has(row)) row.classList.remove('diff-range-highlight');
    }
    for (const row of rows) row.classList.add('diff-range-highlight');
  }

  repainters.set(table, repaint);
  new MutationObserver((mutations) => {
    if (mutations.some(({target}) => target === body ||
      (target instanceof Element && target.matches('.add-comment-left, .add-comment-right, .diff-comment-lines')))) {
      queueMicrotask(repaint);
    }
  }).observe(body, {childList: true, subtree: true});

  function select(range: DiffCommentRange, anchor: Endpoint) {
    setDiffLineSelection(table, range, anchor);
    window.location.hash = selection!.hash;
  }

  function buttonEndpoint(button: HTMLElement): Endpoint {
    return {
      row: button.closest<HTMLTableRowElement>('.diff-review-line')!,
      line: sideLine(button.getAttribute('data-side')!, button.getAttribute('data-idx')!),
    };
  }

  function canComment({row, line}: Endpoint) {
    const side = line[0] === 'L' ? 'left' : 'right';
    return Array.from(row.querySelectorAll(commentButton)).some((button) =>
      button.getAttribute('data-side') === side && !button.classList.contains('tw-invisible'));
  }

  function onSide(endpoint: Endpoint, side: 'old' | 'new') {
    return toEndpoint(endpoint.row.querySelector<HTMLAnchorElement>(`.lines-num-${side} ${lineLink}`)!);
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

  async function openDiffComment({start, end}: DiffCommentRange) {
    const {row, line} = end;
    const side = line[0] === 'L' ? 'left' : 'right';
    const isRange = start.line !== line;
    let commentRow = row.nextElementSibling;
    if (!commentRow?.classList.contains('add-comment')) {
      const cells = split ?
        html`<td class="add-comment-left" colspan="4"></td><td class="add-comment-right" colspan="4"></td>` :
        html`<td class="add-comment-left add-comment-right" colspan="5"></td>`;
      commentRow = createElementFromHTML(html`
        <tr class="add-comment" data-line-type="${row.getAttribute('data-line-type')!}">${htmlRaw(cells)}</tr>
      `);
      row.after(commentRow);
    }

    const cell = commentRow.querySelector<HTMLTableCellElement>(`.add-comment-${side}`)!;
    if (cell.querySelector('.conversation-holder')) return;
    const draft = createElementFromAttrs('div', {class: 'conversation-holder diff-comment-draft'});
    cell.append(draft);
    let form: HTMLFormElement;
    try {
      const response = await performFetchActionRequest(draft, {
        method: 'GET',
        url: table.getAttribute('data-new-comment-url')!,
        loadingIndicator: '$this',
      });
      if (!response) return;
      const content = await response.text();
      if (!draft.isConnected) return;
      draft.append(...createElementFromHTML(content).children);
      form = draft.querySelector<HTMLFormElement>('form')!;
    } finally {
      if (!draft.childElementCount) draft.remove();
    }

    const field = (name: string) => form.elements.namedItem(name) as HTMLInputElement;
    field('line').value = line.slice(1);
    field('start_line').value = isRange ? `${start.line[0] === 'L' ? '-' : ''}${start.line.slice(1)}` : '';
    field('side').value = side === 'left' ? 'previous' : 'proposed';
    field('path').value = table.getAttribute('data-path')!;
    const label = form.querySelector('.diff-comment-lines')!;
    toggleElem(label, isRange);
    if (isRange) {
      const sameSide = start.line[0] === line[0];
      label.textContent = trString(label.getAttribute('data-text-lines')!, sameSide ? start.line.slice(1) : start.line, sameSide ? line.slice(1) : line);
    }
    (await initCodeCommentEditor(form)).focus();
  }

  function extendComment(start: Endpoint, end: Endpoint) {
    const range = commentRange(orderRange(start, end), end);
    select(range, range.start.row === start.row ? range.start : range.end);
    return openDiffComment(range);
  }

  addDelegatedEventListener<HTMLAnchorElement, MouseEvent>(table, 'click', lineLink, (link, e) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    e.preventDefault();
    const end = toEndpoint(link);
    const start = e.shiftKey && selection?.table === table ? selection.anchor : end;
    select(orderRange(start, end), start);
    window.getSelection()!.removeAllRanges();
  });

  addDelegatedEventListener<HTMLButtonElement, MouseEvent>(table, 'click', commentButton, (button, e) => {
    if (button.classList.contains('tw-invisible')) return;
    e.preventDefault();
    if (suppressClick && e.detail > 0) return;
    const endpoint = buttonEndpoint(button);
    const active = selection?.table === table ? selection : undefined;

    if (active?.range.end.row === endpoint.row) {
      if (active.range.end.line === endpoint.line || endpoint.row.matches('.same-code')) {
        const range = commentRange(active.range, endpoint);
        if (range.start.line !== range.end.line) return openDiffComment(range);
      }
      if (active.range.start.line !== endpoint.line || active.range.end.line !== endpoint.line) {
        select({start: endpoint, end: endpoint}, endpoint);
      }
    }
    return openDiffComment({start: endpoint, end: endpoint});
  });

  function pointerTarget(e: PointerEvent): Endpoint | undefined {
    const cell = document.elementFromPoint(e.clientX, e.clientY)?.closest<HTMLTableCellElement>(
      '.lines-num, .lines-escape, .lines-type-marker, .lines-code',
    );
    if (cell?.parentElement?.parentElement !== body || cell.parentElement.classList.contains('tag-code')) return;
    const column = split ? (cell.cellIndex < 4 ? '.lines-code-old' : '.lines-code-new') : '.lines-code';
    const button = cell.parentElement.querySelector<HTMLElement>(`${column} .add-code-comment`);
    return button ? buttonEndpoint(button) : undefined;
  }

  table.addEventListener('pointerdown', (e) => {
    if (e.button !== 0 || !e.isPrimary) return;
    suppressClick = false;
    const button = (e.target as Element).closest<HTMLElement>(commentButton);
    if (!button || button.classList.contains('tw-invisible') || e.shiftKey) return;
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
    addDelegatedEventListener<HTMLElement, PointerEvent | FocusEvent>(table, event, '.conversation-holder', (holder, e) => {
      if (!holder.hasAttribute('data-range-start')) return;
      const target = e.relatedTarget instanceof Element ? e.relatedTarget.closest('.conversation-holder') : null;
      const related = target?.hasAttribute('data-range-start') ? target : null;
      if (related === holder) return;
      const active = event === 'pointerout' || event === 'focusout' ? related : holder;
      if (e instanceof FocusEvent) focused = active;
      else hovered = active;
      repaint();
    });
  }
}

export function codeSuggestionInsertion(value: string, start: number, end: number, source: string) {
  let fenceLength = 3;
  for (const [backticks] of source.matchAll(/`+/g)) {
    fenceLength = Math.max(fenceLength, backticks.length + 1);
  }
  const fence = '`'.repeat(fenceLength);
  const prefix = start > 0 && value[start - 1] !== '\n' ? '\n' : '';
  const suffix = end < value.length && value[end] !== '\n' ? '\n' : '';
  const opening = `${prefix}${fence}suggestion\n${source}`;
  return {text: `${opening}\n${fence}${suffix}`, cursor: start + opening.length};
}

function canInsertCodeSuggestion(form: HTMLFormElement) {
  const start = Number((form.elements.namedItem('start_line') as HTMLInputElement).value);
  return start === 0 ? (form.elements.namedItem('side') as HTMLInputElement).value === 'proposed' : start > 0;
}

function codeSuggestionAnchor(form: HTMLFormElement) {
  if (!canInsertCodeSuggestion(form)) return null;
  const input = (name: string) => (form.elements.namedItem(name) as HTMLInputElement).value;
  return new URLSearchParams({
    path: input('path'),
    start_line: input('start_line'),
    line: input('line'),
    commit_id: input('latest_commit_id'),
  });
}

export function initCodeCommentEditor(form: HTMLFormElement) {
  const container = form.querySelector<HTMLElement>('.combo-markdown-editor')!;
  const button = form.querySelector<HTMLButtonElement>('.markdown-toolbar-extra-button');
  if (button) toggleExtraToolbarButton(button, canInsertCodeSuggestion(form));
  return getComboMarkdownEditor(container) ?? initComboMarkdownEditor(container, button ? {
    easyMDEToolbarButton: button,
    appendPreviewData(data) {
      codeSuggestionAnchor(form)?.forEach((value, key) => { data.append(`code_suggestion_${key}`, value) });
    },
  } : undefined);
}

async function onInsertCodeSuggestion(button: HTMLButtonElement) {
  const form = button.form!;
  const anchor = codeSuggestionAnchor(form)?.toString();
  if (!anchor) return;
  const response = await performFetchActionRequest(button, {
    method: 'GET',
    url: `${form.getAttribute('data-code-suggestion-source-url')!}?${anchor}`,
  });
  if (!response) return;
  const source = await response.arrayBuffer();
  if (!button.isConnected || anchor !== codeSuggestionAnchor(form)?.toString()) return;
  const code = new TextDecoder('utf-8', {ignoreBOM: true}).decode(source);
  getComboMarkdownEditor(button.closest('.combo-markdown-editor'))!.replaceSelection((value, start, end) => codeSuggestionInsertion(value, start, end, code));
}

const batch = new Map<string, {path: string; start: number; end: number; version: string}>();

function updateBatch(batchButton: Element) {
  batchButton.setAttribute('data-modal-comment_ids', Array.from(batch, ([id, item]) => `${id}:${item.version}`).join(','));
  batchButton.querySelector('.code-suggestion-batch-count')!.textContent = String(batch.size);
  toggleElem(batchButton, batch.size > 0);
}

function updateBatchToggle(batchButton: Element, toggle: Element, id: string) {
  toggle.textContent = batchButton.getAttribute(batch.has(id) ? 'data-remove-text' : 'data-add-text')!;
}

export function refreshCodeSuggestionControls(actions: Element | null, controlsHTML: string) {
  if (!actions) return;
  const previousId = actions.firstElementChild?.getAttribute('data-comment-id');
  actions.innerHTML = controlsHTML;
  if (previousId && !actions.firstElementChild && batch.delete(previousId)) {
    updateBatch(document.querySelector('.code-suggestion-batch')!);
  }
}

export function initRepoDiffReview() {
  registerGlobalSelectorFunc('.diff-review-table .diff-line-marker', (marker) => {
    const rel = marker.getAttribute('rel');
    if (!rel || marker.nextElementSibling?.classList.contains('diff-line-link')) return;
    const link = createElementFromAttrs<HTMLAnchorElement>('a', {class: 'diff-line-link tw-absolute tw-inset-0', href: `#${rel}`});
    link.setAttribute('aria-label', parseDiffLineHash(link.hash)!.start);
    marker.parentElement!.classList.add('tw-relative');
    marker.after(link);
  });
  registerGlobalInitFunc('initDiffSelection', initDiffSelection);

  registerGlobalEventFunc('click', 'onInsertCodeSuggestion', onInsertCodeSuggestion);

  registerGlobalInitFunc('initCodeSuggestionControls', (controls) => {
    const batchButton = document.querySelector('.code-suggestion-batch');
    if (!batchButton) return;
    const toggle = controls.querySelector('.code-suggestion-batch-toggle');
    const id = controls.getAttribute('data-comment-id')!;
    const queued = batch.get(id);
    if (queued && (!toggle || queued.version !== controls.getAttribute('data-content-version'))) {
      batch.delete(id);
      updateBatch(batchButton);
    }
    if (!toggle) return;
    updateBatchToggle(batchButton, toggle, id);
    showElem(toggle);
  });

  registerGlobalEventFunc('click', 'toggleCodeSuggestionBatch', (toggle) => {
    const batchButton = document.querySelector('.code-suggestion-batch')!;
    const controls = toggle.closest('.code-suggestion-controls')!;
    const id = controls.getAttribute('data-comment-id')!;
    if (!batch.delete(id)) {
      const item = {
        path: controls.getAttribute('data-path')!,
        start: Number(controls.getAttribute('data-start-line')!),
        end: Number(controls.getAttribute('data-end-line')!),
        version: controls.getAttribute('data-content-version')!,
      };
      if (Array.from(batch.values()).some((other) => item.path === other.path && item.start <= other.end && other.start <= item.end)) {
        showErrorToast(batchButton.getAttribute('data-overlap-text')!);
        return;
      }
      batch.set(id, item);
    }
    updateBatch(batchButton);
    updateBatchToggle(batchButton, toggle, id);
  });
}
