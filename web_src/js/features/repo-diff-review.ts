import {registerGlobalEventFunc, registerGlobalInitFunc, registerGlobalSelectorFunc} from '../modules/observer.ts';
import {performFetchActionRequest} from '../modules/fetch-action.ts';
import {trString} from '../modules/i18n.ts';
import {showErrorToast} from '../modules/toast.ts';
import {
  addDelegatedEventListener, createElementFromAttrs, createElementFromHTML, hideElem, showElem, toggleElem,
} from '../utils/dom.ts';
import {html, htmlRaw} from '../utils/html.ts';
import {getComboMarkdownEditor, initComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';

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
  return {row: link.closest('tr')!, line: parseDiffLineHash(link.hash)!.start};
}

function orderRange(start: Endpoint, end: Endpoint): DiffCommentRange {
  if (start.row === end.row ? start.line[0] > end.line[0] :
    start.row.compareDocumentPosition(end.row) & Node.DOCUMENT_POSITION_PRECEDING) {
    return {start: end, end: start};
  }
  return {start, end};
}

export function findDiffLine(table: Element, line: DiffLine) {
  return table.querySelector<HTMLAnchorElement>(`${lineLink}[href$="${CSS.escape(line)}"]`);
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

export function resolveDiffRange(table: Element, start: DiffLine, end: DiffLine): DiffCommentRange | undefined {
  const first = findDiffLine(table, start);
  const last = findDiffLine(table, end);
  if (!first || !last) return;
  return orderRange(toEndpoint(first), toEndpoint(last));
}

export function isDiffLineSelected(hash: string) {
  return selection?.hash === hash && selection.table.isConnected;
}

export function setDiffLineSelection(range?: DiffCommentRange, anchor = range?.start) {
  const previous = selection?.table;
  if (range) {
    const {start, end} = range;
    selection = {
      table: start.row.closest('table')!, range, anchor: anchor!,
      hash: `#${start.row.closest('.diff-file-box')!.id}${start.line}${start.line === end.line ? '' : `-${end.line}`}`,
    };
  } else {
    selection = undefined;
  }
  if (previous && previous !== selection?.table) repainters.get(previous)?.();
  if (selection) repainters.get(selection.table)?.();
}

function signedDiffLine(number: number): DiffLine {
  return `${number < 0 ? 'L' : 'R'}${Math.abs(number)}`;
}

function sideLine(side: string, line: string): DiffLine {
  return `${side === 'previous' || side === 'left' ? 'L' : 'R'}${Number(line)}`;
}

function initDiffSelection(table: HTMLTableElement) {
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
      if (start) paint(resolveDiffRange(table, signedDiffLine(start), sideLine(field('side'), field('line'))));
    }
    for (const holder of new Set([hovered, focused])) {
      if (!holder || !table.contains(holder)) continue;
      paint(resolveDiffRange(table, signedDiffLine(Number(holder.getAttribute('data-range-start'))),
        signedDiffLine(Number(holder.getAttribute('data-range-end')))));
    }
    for (const row of body.querySelectorAll(':scope > .diff-range-highlight')) {
      if (!rows.has(row)) row.classList.remove('diff-range-highlight');
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
    setDiffLineSelection(range, anchor);
    window.location.hash = selection!.hash;
  }

  function buttonEndpoint(button: HTMLElement): Endpoint {
    return {
      row: button.closest('tr')!,
      line: sideLine(button.getAttribute('data-side')!, button.getAttribute('data-idx')!),
    };
  }

  function canComment({row, line}: Endpoint) {
    const side = line[0] === 'L' ? 'left' : 'right';
    return row.querySelector(`${commentButton}[data-side="${CSS.escape(side)}"]:not(.tw-invisible)`) !== null;
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

    const cell = commentRow.querySelector(`.add-comment-${side}`)!;
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
    if (isRange) {
      const label = form.querySelector('.diff-comment-lines')!;
      showElem(label);
      const sameSide = start.line[0] === line[0];
      label.textContent = trString(label.getAttribute('data-text-lines')!, sameSide ? start.line.slice(1) : start.line, sameSide ? line.slice(1) : line);
    }
    (await initCodeCommentEditor(form)).focus();
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
    const single = {start: endpoint, end: endpoint};
    const active = selection?.range;
    if (active?.end.row !== endpoint.row) return openDiffComment(single);

    const range = active.end.line === endpoint.line || endpoint.row.matches('.same-code') ? commentRange(active, endpoint) : single;
    if (range.start.line === range.end.line && (active.start.line !== endpoint.line || active.end.line !== endpoint.line)) {
      select(single, endpoint);
    }
    return openDiffComment(range);
  });

  function pointerTarget(e: PointerEvent): Endpoint | undefined {
    const cell = document.elementFromPoint(e.clientX, e.clientY)?.closest('td');
    if (cell?.parentElement?.parentElement !== body || cell.parentElement.matches('.tag-code, .add-comment')) return;
    const column = split ? (cell.cellIndex < 4 ? '.lines-code-old' : '.lines-code-new') : '.lines-code';
    const button = cell.parentElement.querySelector<HTMLElement>(`${column} ${commentButton}`);
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
      if (end && suppressClick) {
        const range = commentRange(orderRange(start, end), end);
        select(range, range.start.row === start.row ? range.start : range.end);
        openDiffComment(range);
      } else {
        repaint();
      }
    });
  }

  const savedRange = '.conversation-holder[data-range-start]';
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

function codeSuggestionAnchor(form: HTMLFormElement) {
  const input = (name: string) => (form.elements.namedItem(name) as HTMLInputElement).value;
  const startLine = input('start_line');
  const start = Number(startLine);
  if (!(start > 0 || (start === 0 && input('side') === 'proposed'))) return null;
  return new URLSearchParams({
    path: input('path'),
    start_line: startLine,
    line: input('line'),
    commit_id: input('latest_commit_id'),
  });
}

export function initCodeCommentEditor(form: HTMLFormElement) {
  const container = form.querySelector<HTMLElement>('.combo-markdown-editor')!;
  const button = form.querySelector<HTMLButtonElement>('.markdown-toolbar-extra-button');
  const anchor = button && codeSuggestionAnchor(form);
  if (button && !anchor) hideElem(button.parentElement!);
  return getComboMarkdownEditor(container) ?? initComboMarkdownEditor(container, anchor ? {
    easyMDEToolbarButton: button,
    appendPreviewData(data) {
      for (const [key, value] of anchor) data.append(`code_suggestion_${key}`, value);
    },
  } : undefined);
}

async function onInsertCodeSuggestion(button: HTMLButtonElement) {
  const form = button.form!;
  let code = '';
  if (!form.classList.contains('comment-form')) {
    const anchor = codeSuggestionAnchor(form);
    if (!anchor) return;
    const response = await performFetchActionRequest(button, {
      method: 'GET',
      url: `${form.getAttribute('data-code-suggestion-source-url')!}?${anchor}`,
    });
    if (!response) return;
    const source = await response.arrayBuffer();
    if (!button.isConnected) return;
    code = new TextDecoder('utf-8', {ignoreBOM: true}).decode(source);
  }
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
  registerGlobalSelectorFunc('.diff-review-table td.lines-num > span[rel^="diff-"]', (marker) => {
    if (marker.nextElementSibling?.matches(lineLink)) return;
    const href = `#${marker.getAttribute('rel')!}`;
    marker.parentElement!.classList.add('tw-relative');
    marker.after(createElementFromAttrs('a', {
      class: 'diff-line-link tw-absolute tw-inset-0', href, 'aria-label': parseDiffLineHash(href)!.start,
    }));
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
