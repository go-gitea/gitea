import {diffTreeStore} from '../modules/diff-file.ts';
import {performFetchActionTrigger} from '../modules/fetch-action.ts';
import {initAsyncHashChangeListeners} from '../modules/hashchange.ts';
import {initGlobalSelectorObserver} from '../modules/observer.ts';
import {createElementFromHTML} from '../utils/dom.ts';
import {initRepoDiffView} from './repo-diff.ts';
import {
  formatDiffLineHash, normalizeDiffCommentRange, parseDiffLineHash, registerRepoDiffSelection, repaintDiffRanges,
  resolveDiffRange, setDiffLineSelection, type DiffLine,
} from './repo-diff-selection.ts';

vi.mock('../modules/fetch-action.ts', {spy: true});

const fileHash = 'a'.repeat(40);
const prefix = `#diff-${fileHash}`;
const originalUrl = window.location.href;
const originalScrollRestoration = window.history.scrollRestoration;
const openComment = vi.fn<Parameters<typeof registerRepoDiffSelection>[0]>();

beforeAll(async () => {
  document.body.innerHTML = '<div id="diff-file-boxes"></div>';
  registerRepoDiffSelection(openComment);
  initRepoDiffView();
  initGlobalSelectorObserver(null);
  await initAsyncHashChangeListeners();
});

beforeEach(() => openComment.mockReset().mockResolvedValue(undefined));

afterEach(() => {
  setDiffLineSelection();
  vi.restoreAllMocks();
  document.body.replaceChildren();
  window.history.replaceState(null, '', originalUrl);
  window.history.scrollRestoration = originalScrollRestoration;
});

function diffRow(left: number, right: number, type = left && right ? 'same' : left ? 'del' : 'add') {
  const columns = (line: number, side: 'L' | 'R', column: string, commentSide: string) => line ?
    `<td data-diff-column="${column}"><a class="diff-line-link" href="${prefix}${side}${line}" tabindex="-1" data-global-click="onSelectDiffLine">${line}</a></td>
     <td data-diff-column="${column}"><button class="add-code-comment" data-side="${commentSide}" data-global-click="onAddDiffComment"></button></td>` :
    '<td></td><td></td>';
  return `<tr class="${type}-code" data-line-type="${type}">${columns(left, 'L', 'old', 'left')}${columns(right, 'R', 'new', 'right')}</tr>`;
}

function gap(direction: string, last: number, next: number, hunkSize: number) {
  const url = `/excerpt?direction=${direction}&last_left=${last}&last_right=${last}&left=${next}&right=${next}&left_hunk_size=${hunkSize}&right_hunk_size=${hunkSize}`;
  return `<tr class="tag-code"><td><button class="code-expander-button" data-fetch-sync="$closest(tr)" data-fetch-url="${url}"></button></td></tr>`;
}

function diffBox(rows: string, folded = false) {
  return `<div class="diff-file-box" id="diff-${fileHash}" data-folded="${folded}"><button class="fold-file"></button>
    <div class="code-diff"><table data-global-init="initDiffSelection"><tbody>${rows}</tbody></table></div></div>`;
}

async function mountDiff(rows: string) {
  document.body.innerHTML = diffBox(rows);
  const table = document.querySelector('table')!;
  await vi.waitFor(() => expect(table).toHaveProperty('_giteaGlobalInited', true));
  return table;
}

function clickLine(line: DiffLine, shiftKey = false) {
  const link = document.querySelector(`a[href="${CSS.escape(prefix + line)}"]`)!;
  link.dispatchEvent(new MouseEvent('click', {bubbles: true, cancelable: true, shiftKey}));
}

function highlighted() {
  return [...document.querySelectorAll('tr.diff-range-highlight')];
}

test.each([['L5', 'L5', 'L5'], ['R5-R8', 'R5', 'R8'], ['L6-R7', 'L6', 'R7']] as const)(
  'round-trips diff line hash %s', (fragment, start, end) => {
    expect(parseDiffLineHash(prefix + fragment)).toEqual({fileHash, start, end});
    expect(formatDiffLineHash({fileHash, start, end})).toBe(prefix + fragment);
  },
);

test('ignores malformed diff line hashes', () => {
  expect(parseDiffLineHash(`${prefix}R5-8`)).toBeUndefined();
});

test('falls back to the clicked side of a split replacement', async () => {
  const table = await mountDiff(diffRow(5, 5) + diffRow(6, 7, 'del'));
  const [, replacement] = table.rows;
  const left = {row: replacement, line: 'L6'}, right = {row: replacement, line: 'R7'};
  expect(resolveDiffRange(table, {fileHash, start: 'R7', end: 'L6'})).toEqual({start: left, end: right, rows: [replacement]});

  clickLine('L5');
  clickLine('L6', true);
  replacement.querySelector<HTMLButtonElement>('button[data-side="right"]')!.click();
  expect(window.location.hash).toBe(`${prefix}R7`);
  expect(highlighted()).toEqual([replacement]);
  expect(openComment).toHaveBeenLastCalledWith({start: right, end: right});

  clickLine('R7');
  replacement.querySelector('button[data-side="left"]')!.dispatchEvent(new MouseEvent('click', {bubbles: true, shiftKey: true}));
  expect(window.location.hash).toBe(`${prefix}L6`);
  expect(openComment).toHaveBeenLastCalledWith({start: left, end: left}, undefined);
});

test('selects a reversed cross-hunk range and opens a mixed comment on its last row', async () => {
  const table = await mountDiff(diffRow(6, 0) + gap('up', 7, 30, 1) + diffRow(0, 30));
  const [first, , last] = table.rows;
  clickLine('R30');
  clickLine('L6', true);
  expect(window.location.hash).toBe(`${prefix}L6-R30`);
  expect(highlighted()).toEqual([first, last]);
  last.querySelector('button')!.click();
  expect(openComment).toHaveBeenCalledWith({start: {row: first, line: 'L6'}, end: {row: last, line: 'R30'}}, undefined);
});

test.each([
  ['R5', 'L25', 'L4', 'L25'],
  ['R6', 'L5', 'R6', 'R7'],
  ['R6', 'L25', 'L25', 'L25'],
  ['L4', 'R5', 'R5', 'R5'],
] as const)('normalizes comment endpoints %s to %s', (first, last, start, end) => {
  const table = document.createElement('table');
  table.innerHTML = `<tbody>${diffRow(4, 5)}${diffRow(0, 6)}${diffRow(5, 7)}${diffRow(25, 0)}</tbody>`;
  const range = resolveDiffRange(table, {fileHash, start: first, end: last})!;
  const normalized = normalizeDiffCommentRange(range, range.end, fileHash);
  expect([normalized.start.line, normalized.end.line]).toEqual([start, end]);
});

test.each([
  ['open draft', 'R7'],
  ['removed draft', 'R30'],
  ['gutter selection', 'L6'],
] as const)('shift-click on plus with %s', async (source, start) => {
  const table = await mountDiff(diffRow(6, 0) + diffRow(0, 7) + diffRow(0, 30));
  const [first, middle, last] = table.rows;
  openComment.mockImplementationOnce(() => {
    middle.after(createElementFromHTML('<tr class="add-comment"><td class="add-comment-right"><div class="diff-comment-draft"></div></td></tr>'));
    return Promise.resolve();
  });
  middle.querySelector('button')!.click();
  const draft = table.querySelector('.diff-comment-draft')!;
  if (source === 'removed draft') draft.closest('tr')!.remove();
  else if (source === 'gutter selection') clickLine('L6');

  last.querySelector('button')!.dispatchEvent(new MouseEvent('click', {bubbles: true, shiftKey: true}));
  const selection = {start: {row: {R7: middle, R30: last, L6: first}[start], line: start}, end: {row: last, line: 'R30'}};
  await vi.waitFor(() => expect(openComment).toHaveBeenCalledTimes(2));
  const [actual, reusedDraft] = openComment.mock.lastCall!;
  expect(actual).toEqual(selection);
  expect(reusedDraft).toBe(source === 'open draft' ? draft : undefined);
  expect(window.location.hash).toBe(start === 'R30' ? '' : `${prefix}${start}-R30`);
});

test('repaints a moved draft and highlights a saved thread while hovered or focused', async () => {
  const table = await mountDiff(diffRow(8, 0) + diffRow(9, 11) + gap('up', 11, 30, 1) + diffRow(30, 30));
  const [first, middle, , last] = table.rows;
  const draft = createElementFromHTML<HTMLTableRowElement>(`<tr class="add-comment"><td><div class="diff-comment-draft"><form>
    <input name="start_line" value="8"><input name="start_side" value="previous"><input name="line" value="11"><input name="side" value="proposed">
  </form></div></td></tr>`);
  middle.after(draft);
  repaintDiffRanges(table);
  expect(highlighted()).toEqual([first, middle]);
  last.after(draft);
  draft.querySelector<HTMLInputElement>('input[name="line"]')!.value = '30';
  repaintDiffRanges(table);
  expect(highlighted()).toEqual([first, middle, last]);

  draft.cells[0].innerHTML = '<div class="conversation-holder" data-start-idx="-8" data-idx="30"><button></button></div>';
  repaintDiffRanges(table);
  expect(highlighted()).toEqual([]);
  const button = draft.querySelector('button')!;
  button.dispatchEvent(new MouseEvent('pointerover', {bubbles: true}));
  expect(highlighted()).toEqual([first, middle, last]);
  button.focus();
  button.dispatchEvent(new MouseEvent('pointerout', {bubbles: true}));
  expect(highlighted()).toEqual([first, middle, last]);
  button.parentElement!.setAttribute('data-start-idx', '-15');
  repaintDiffRanges(table);
  expect(highlighted()).toEqual([last]);
  button.blur();
  expect(highlighted()).toEqual([]);
});

test('line navigation clears a file anchor and Back restores it', async () => {
  const table = await mountDiff(diffRow(5, 5));
  const box = table.closest('.diff-file-box')!;
  window.location.hash = prefix;
  await vi.waitFor(() => expect(box.matches(':target')).toBe(true));

  clickLine('R5');
  expect(window.location.hash).toBe(`${prefix}R5`);
  expect(box.matches(':target')).toBe(false);
  expect(highlighted()).toEqual([...table.rows]);

  window.history.back();
  await vi.waitFor(() => {
    expect(box.matches(':target')).toBe(true);
    expect(highlighted()).toEqual([]);
  });
});

test('loads a folded diff and expands both hidden range endpoints before scrolling', async () => {
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
  const box = diffBox(gap('up', 0, 5, 1) + diffRow(5, 5) + gap('down', 5, 8, 0), true);
  const request = vi.spyOn(window, 'fetch').mockResolvedValueOnce(new Response(`<div id="diff-file-boxes">${box}</div>`));
  const expand = vi.mocked(performFetchActionTrigger).mockImplementation((button) => {
    const line = button.getAttribute('data-fetch-url')!.includes('direction=up') ? 2 : 8;
    button.closest('tr')!.replaceWith(createElementFromHTML(diffRow(line, line)));
    return Promise.resolve();
  });
  const scroll = vi.spyOn(HTMLElement.prototype, 'scrollIntoView').mockImplementation(() => {});
  window.history.replaceState(null, '', '?style=split&whitespace=ignore-all');
  window.location.hash = `${prefix}R8-L2`;
  await vi.waitFor(() => expect(scroll).toHaveBeenCalled());

  expect(request.mock.calls.map(([url]) => new URL(url as string).search)).toEqual(['?skip-to=next&file-only=true&style=split&whitespace=ignore-all']);
  expect(expand).toHaveBeenCalledTimes(2);
  const table = document.querySelector('table')!;
  expect(table.closest('.diff-file-box')!.getAttribute('data-folded')).toBe('false');
  expect(highlighted()).toEqual([...table.rows]);
  expect(diffTreeStore().selectedFileHash).toBe(fileHash);
  expect(document.activeElement).toBe(table.rows[0].querySelector(`a[href="${CSS.escape(`${prefix}L2`)}"]`));
  expect(scroll.mock.contexts).toEqual([table.rows[0]]);
});
