import {initGlobalSelectorObserver} from '../modules/observer.ts';
import {initAsyncHashChangeListeners} from '../modules/hashchange.ts';
import {performFetchActionTrigger} from '../modules/fetch-action.ts';
import {diffTreeStore} from '../modules/diff-file.ts';
import {createElementFromHTML} from '../utils/dom.ts';
import {initRepoDiffView} from './repo-diff.ts';
import {
  findDiffLine, formatDiffLineHash, normalizeDiffCommentRange, parseDiffLineHash, registerRepoDiffSelection, repaintDiffRanges,
  resolveDiffRange, setDiffLineSelection, type DiffLine,
} from './repo-diff-selection.ts';

vi.mock('../modules/fetch-action.ts', {spy: true});

const fileHash = 'a'.repeat(40);
const originalUrl = window.location.href;
const originalScrollRestoration = window.history.scrollRestoration;
const openComment = vi.fn<Parameters<typeof registerRepoDiffSelection>[0]>();

beforeAll(() => {
  document.body.innerHTML = '<div id="diff-file-boxes"></div>';
  registerRepoDiffSelection(openComment);
  initRepoDiffView();
  initGlobalSelectorObserver(null);
});

beforeEach(() => {
  openComment.mockReset().mockResolvedValue(undefined);
});

afterEach(() => {
  setDiffLineSelection();
  vi.restoreAllMocks();
  document.body.replaceChildren();
  window.history.replaceState(null, '', originalUrl);
  window.history.scrollRestoration = originalScrollRestoration;
});

function hash(start: DiffLine, end = start) {
  return formatDiffLineHash({fileHash, start, end});
}

function diffRow(left: number, right: number, type: string, split: boolean) {
  const gutter = (column: string, side: 'L' | 'R', line: number) =>
    `<td data-diff-column="${column}">${line ?
      `<a class="diff-line-link" href="${hash(`${side}${line}`)}" data-global-click="onSelectDiffLine">${line}</a>` : ''}</td>`;
  const button = (side: string) => `<button class="add-code-comment" data-side="${side}" data-global-click="onAddDiffComment"></button>`;
  return `<tr class="${type}-code" data-line-type="${type}">${split ?
    `${gutter('old', 'L', left)}<td data-diff-column="old">${left ? button('left') : ''}</td>
     ${gutter('new', 'R', right)}<td data-diff-column="new">${right ? button('right') : ''}</td>` :
    `${gutter('unified', 'L', left)}${gutter('unified', 'R', right)}<td data-diff-column="unified">${button(right ? 'right' : 'left')}</td>`}</tr>`;
}

function diffBox(rows: Array<[number, number, string] | 'hunk'>, split: boolean) {
  const contents = rows.map((row) => row === 'hunk' ? '<tr class="tag-code"><td></td></tr>' : diffRow(...row, split)).join('');
  return createElementFromHTML(`<div class="diff-file-box" id="diff-${fileHash}"><div class="code-diff">
    <table data-global-init="initDiffSelection"><tbody>${contents}</tbody></table></div></div>`);
}

async function mountDiff(rows: Parameters<typeof diffBox>[0], split = false) {
  const box = diffBox(rows, split);
  document.body.replaceChildren(box);
  await vi.waitFor(() => expect(box.querySelector('a')!.getAttribute('tabindex')).toBe('-1'));
  return box.querySelector('table')!;
}

function resolve(table: HTMLTableElement, start: DiffLine, end: DiffLine) {
  return resolveDiffRange(table, {fileHash, start, end})!;
}

function paintedRows(table: HTMLTableElement) {
  return [...table.rows].filter((row) => row.matches('.diff-range-highlight'));
}

function expander(direction: string, last: number, next: number, hunkSize: number) {
  const url = `/excerpt?direction=${direction}&last_left=${last}&last_right=${last}&left=${next}&right=${next}&left_hunk_size=${hunkSize}&right_hunk_size=${hunkSize}`;
  return `<button class="code-expander-button" data-fetch-sync="$closest(tr)" data-fetch-url="${url}"></button>`;
}

function shiftClick(target: Element) {
  target.dispatchEvent(new MouseEvent('click', {bubbles: true, cancelable: true, shiftKey: true}));
}

test('parses and formats single, same-side and mixed diff line hashes', () => {
  for (const [fragment, start, end] of [['L5', 'L5', 'L5'], ['R5-R8', 'R5', 'R8'], ['L6-R7', 'L6', 'R7']] as const) {
    expect(parseDiffLineHash(`#diff-${fileHash}${fragment}`)).toEqual({fileHash, start, end});
    expect(hash(start, end)).toBe(`#diff-${fileHash}${fragment}`);
  }
  for (const fragment of ['', 'R0', 'L3-4', 'L3-R4-extra']) {
    expect(parseDiffLineHash(`#diff-${fileHash}${fragment}`)).toBeUndefined();
  }
});

test('highlights whole split rows and falls back to the clicked plus side of a replacement', async () => {
  const table = await mountDiff([[5, 5, 'same'], [6, 7, 'del'], [7, 8, 'same']], true);
  const [, changed, last] = table.rows;
  expect(resolve(table, 'R7', 'L6')).toEqual({
    start: {row: changed, line: 'L6'}, end: {row: changed, line: 'R7'}, rows: [changed],
  });
  findDiffLine(table, fileHash, 'R8')!.click();
  shiftClick(findDiffLine(table, fileHash, 'L6')!);
  expect(window.location.hash).toBe(hash('L6', 'R8'));
  expect(paintedRows(table)).toEqual([changed, last]);
  findDiffLine(table, fileHash, 'L5')!.click();
  shiftClick(findDiffLine(table, fileHash, 'L6')!);
  changed.querySelector<HTMLButtonElement>('button[data-side="right"]')!.click();
  expect(window.location.hash).toBe(hash('R7'));
  expect(paintedRows(table)).toEqual([changed]);
  const endpoint = {row: changed, line: 'R7'};
  expect(openComment).toHaveBeenLastCalledWith({start: endpoint, end: endpoint});

  findDiffLine(table, fileHash, 'R7')!.click();
  shiftClick(changed.querySelector('button[data-side="left"]')!);
  expect(window.location.hash).toBe(hash('L6'));
  const left = {row: changed, line: 'L6'};
  expect(openComment).toHaveBeenLastCalledWith({start: left, end: left}, undefined);
});

test('resolves reversed cross-hunk ranges and normalizes comment endpoints', () => {
  const table = diffBox([[4, 5, 'same'], [0, 6, 'add'], [5, 7, 'same'], 'hunk', [25, 0, 'del'], [0, 30, 'add']], false).querySelector('table')!;
  const [first, added, context, , removed, last] = table.rows;
  expect(resolve(table, 'R30', 'L4')).toEqual({
    start: {row: first, line: 'L4'}, end: {row: last, line: 'R30'}, rows: [first, added, context, removed, last],
  });
  expect(resolveDiffRange(table, {fileHash, start: 'R15', end: 'R30'})).toBeUndefined();
  const cases: Array<[DiffLine, DiffLine, DiffLine, DiffLine]> = [
    ['R5', 'R30', 'R5', 'R30'],
    ['R5', 'L25', 'L4', 'L25'],
    ['R6', 'L5', 'R6', 'R7'],
    ['R6', 'L25', 'L25', 'L25'],
    ['L4', 'R5', 'R5', 'R5'],
  ];
  for (const [first, last, start, end] of cases) {
    const range = resolve(table, first, last);
    const normalized = normalizeDiffCommentRange(range, range.end, fileHash);
    expect([normalized.start.line, normalized.end.line]).toEqual([start, end]);
  }
});

test('opens a mixed comment on the selected last row', async () => {
  const table = await mountDiff([[6, 0, 'del'], 'hunk', [0, 30, 'add']]);
  const [first, , last] = table.rows;
  first.querySelector('a')!.click();
  shiftClick(last.querySelector('a')!);
  last.querySelector('button')!.click();
  expect(openComment).toHaveBeenLastCalledWith({start: {row: first, line: 'L6'}, end: {row: last, line: 'R30'}}, undefined);
});

test.each([
  {name: 'extends the open plus draft', start: 'R7', reuseDraft: true},
  {name: 'starts a single-line draft after removal', start: 'R30', removeDraft: true},
  {name: 'starts from the explicit gutter selection', start: 'L6', selectGutter: true},
] as const)('shift-click on plus $name', async ({start, ...options}) => {
  const table = await mountDiff([[6, 0, 'del'], [0, 7, 'add'], [0, 30, 'add']]);
  const [first, middle, last] = table.rows;
  const draftRow = createElementFromHTML('<tr class="add-comment"><td class="add-comment-right"><div class="diff-comment-draft"></div></td></tr>');
  openComment.mockImplementationOnce(() => {
    middle.after(draftRow);
    return Promise.resolve();
  });
  middle.querySelector('button')!.click();
  if ('removeDraft' in options) draftRow.remove();
  if ('selectGutter' in options) first.querySelector('a')!.click();
  const opened = Promise.withResolvers<Parameters<typeof openComment>>();
  openComment.mockImplementationOnce((...args) => {
    opened.resolve(args);
    return Promise.resolve();
  });
  shiftClick(last.querySelector('button')!);
  const range = resolve(table, start, 'R30');
  expect(window.location.hash).toBe(start === 'R30' ? '' : hash(start, 'R30'));
  const [selection, draft] = await opened.promise;
  expect(selection).toEqual({start: range.start, end: range.end});
  expect(draft).toBe('reuseDraft' in options ? draftRow.querySelector('.diff-comment-draft') : undefined);
});

test('repaints a moved draft and highlights a saved mixed cross-hunk thread on hover or focus', async () => {
  const table = await mountDiff([[8, 0, 'del'], [9, 11, 'same'], 'hunk', [30, 30, 'same']], true);
  const [first, middle, gap, last] = table.rows;
  gap.cells[0].innerHTML = expander('up', 11, 30, 1);
  const oldRow = table.tBodies[0].insertRow(middle.sectionRowIndex + 1);
  oldRow.className = 'add-comment';
  const newRow = table.tBodies[0].insertRow(last.sectionRowIndex + 1);
  newRow.className = 'add-comment';
  const oldCell = oldRow.insertCell(), newCell = newRow.insertCell();
  oldCell.innerHTML = `<div class="conversation-holder diff-comment-draft"><form>
    <input name="start_line" value="8"><input name="start_side" value="previous"><input name="line" value="11"><input name="side" value="proposed">
  </form></div>`;

  repaintDiffRanges(table);
  expect(paintedRows(table)).toEqual([first, middle]);
  newCell.append(oldCell.firstElementChild!);
  newCell.querySelector<HTMLInputElement>('input[name="line"]')!.value = '30';
  repaintDiffRanges(table);
  expect(paintedRows(table)).toEqual([first, middle, last]);

  newCell.innerHTML = '<div class="conversation-holder" data-start-idx="-8" data-idx="30"><button></button></div>';
  repaintDiffRanges(table);
  expect(paintedRows(table)).toEqual([]);
  const button = newCell.querySelector('button')!;
  button.dispatchEvent(new MouseEvent('pointerover', {bubbles: true}));
  expect(paintedRows(table)).toEqual([first, middle, last]);
  button.focus();
  button.dispatchEvent(new MouseEvent('pointerout', {bubbles: true}));
  expect(paintedRows(table)).toEqual([first, middle, last]);
  button.parentElement!.setAttribute('data-start-idx', '-15');
  repaintDiffRanges(table);
  expect(paintedRows(table)).toEqual([last]);
  button.blur();
  expect(paintedRows(table)).toEqual([]);
});

test('loads the linked file and expands both hidden endpoints of a reversed range before scrolling', async () => {
  const box = diffBox(['hunk', [5, 5, 'same'], 'hunk'], true);
  const [top, , tail] = box.querySelector('table')!.rows;
  top.cells[0].innerHTML = expander('up', 0, 5, 1);
  tail.cells[0].innerHTML = expander('down', 5, 8, 0);
  const nextPage = `<div id="diff-file-boxes">${box.outerHTML}</div>`;
  document.body.innerHTML = `<div id="diff-file-boxes"><div id="diff-incomplete">
    <button id="diff-show-more-files" data-href="?skip-to=next&amp;file-only=true"></button>
  </div></div>`;
  window.config.pageData.DiffFileTree = {
    TreeRoot: {FullName: '', OldFullName: '', DisplayName: '', NameHash: '', DiffStatus: '', EntryMode: 'tree', IsViewed: false, FileIcon: '', Children: []},
  };
  onTestFinished(() => {
    diffTreeStore().selectedFileHash = '';
    delete window.config.pageData.DiffFileTree;
  });
  const request = vi.spyOn(window, 'fetch').mockResolvedValueOnce(new Response(nextPage));
  const expand = vi.mocked(performFetchActionTrigger).mockImplementation((button) => {
    const line = button.getAttribute('data-fetch-url')!.includes('direction=up') ? 2 : 8;
    button.closest('tr')!.replaceWith(createElementFromHTML(diffRow(line, line, 'same', true)));
    return Promise.resolve();
  });
  const scroll = vi.spyOn(HTMLElement.prototype, 'scrollIntoView').mockImplementation(() => {});
  window.history.replaceState(null, '', `?style=split&whitespace=ignore-all${hash('R8', 'L2')}`);

  await initAsyncHashChangeListeners();

  expect(request).toHaveBeenCalledTimes(1);
  expect(new URL(request.mock.calls[0][0] as string).search).toBe('?skip-to=next&file-only=true&style=split&whitespace=ignore-all');
  expect(expand).toHaveBeenCalledTimes(2);
  expect(expand.mock.calls.map(([button]) => button.getAttribute('data-fetch-url'))).toEqual(expect.arrayContaining([
    top.querySelector('button')!.getAttribute('data-fetch-url'), tail.querySelector('button')!.getAttribute('data-fetch-url'),
  ]));
  const table = document.querySelector('table')!;
  expect(paintedRows(table)).toEqual([...table.rows]);
  expect(diffTreeStore().selectedFileHash).toBe(fileHash);
  expect(document.activeElement).toBe(findDiffLine(table, fileHash, 'L2'));
  expect(scroll.mock.contexts).toEqual([table.rows[0]]);

  window.history.replaceState(null, '', originalUrl);
  await initAsyncHashChangeListeners();
  expect(diffTreeStore().selectedFileHash).toBe('');
  findDiffLine(table, fileHash, 'R5')!.click();
  expect(diffTreeStore().selectedFileHash).toBe(fileHash);
});
