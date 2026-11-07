import {DiffComments, type CommentsHost} from './comments.ts';
import {DiffFile} from './entry.ts';
import {createElementFromAttrs, createElementFromHTML} from '../../utils/dom.ts';
import {parseHunks} from './parse.ts';
import {gapOfLine} from './expand.ts';
import type {ViewerComment} from './types.ts';

test('threadsOf', () => {
  // a changed line 5 of a renamed file, lines 1-2 are hidden above the hunk
  const entry = new DiffFile({name: 'new.txt', oldName: 'old.txt', nameHash: 'x', type: 'renamed', tabSize: 4}, parseHunks('@@ -3,3 +3,3 @@\n a\n-b\n+B\n c\n'), false, false);
  const host = {
    config: {commentsUrl: '/comments'},
    findEntry: () => entry,
    hiddenGap: (_entry: DiffFile, side: 'old' | 'new', line: number) => gapOfLine(entry.gaps(true), side, line),
  } as unknown as CommentsHost;
  const comments = new DiffComments(host);
  const comment = (id: number, path: string, side: 'old' | 'new', line: number) => ({id, path, side, line} as ViewerComment);
  comments.byPath = new Map([
    ['new.txt', [comment(1, 'new.txt', 'new', 4), comment(2, 'new.txt', 'old', 4), comment(3, 'new.txt', 'new', 4), comment(4, 'new.txt', 'new', 1), comment(5, 'new.txt', 'new', 99), comment(7, 'new.txt', 'new', 98)]],
    ['old.txt', [comment(6, 'old.txt', 'old', 3)]],
  ]);
  const threads = [...comments.threadsOf(entry).values()].map(({target, comments}) => `${target.side}:${target.line}=${comments.map((c) => c.id).join(',')}`);
  // both sides of a line are separate threads, the hidden line 1 waits for its gap, lines 98 and 99 outside the diff stay separate, the old name counts
  expect(threads).toEqual(['new:4=1,3', 'old:4=2', 'new:99=5', 'new:98=7', 'old:3=6']);
});

test('renderAnnotations', () => {
  const file = (name: string, lfs?: boolean) => ({name, oldName: name, nameHash: 'y', type: 'modified' as const, tabSize: 4, lfs: lfs ? {new: {size: 1, exists: true}} : undefined});
  const hunks = () => parseHunks('@@ -1,2 +1,2 @@\n a\n-b\n+B\n');
  const entries = [new DiffFile(file('a.txt'), hunks(), false, false), new DiffFile(file('big.bin', true), hunks(), false, false)];
  const host = {config: {isSplit: false}, entries, findEntry: (path: string) => entries.find((e) => e.file.name === path), hiddenGap: () => null} as unknown as CommentsHost;
  const comments = new DiffComments(host);
  comments.buildConversation = () => createElementFromAttrs('div', {class: 'diff-conversation'});
  comments.byPath = new Map([['a.txt', [{id: 1, path: 'a.txt', side: 'new', line: 2} as ViewerComment]], ['big.bin', [{id: 2, path: 'big.bin', side: 'new', line: 2} as ViewerComment]]]);
  const table = '<table><tbody><tr data-new="1"></tr><tr data-new="2"></tr></tbody></table>';
  const rowsOf = (entry: DiffFile) => Array.from(entry.element!.querySelectorAll('tbody > tr'), (row) => row.className || row.getAttribute('data-new'));

  entries[0].element = createElementFromHTML(`<div><div class="code-diff">${table}</div></div>`);
  comments.renderAnnotations(entries[0]);
  expect(rowsOf(entries[0])).toEqual(['1', '2', 'add-comment']);

  // the lines of an LFS file are its pointer's, the shown object diff has others, so the thread goes above
  entries[1].lfsState = 'diff';
  entries[1].element = createElementFromHTML(`<div><div class="code-diff">${table}</div></div>`);
  comments.renderAnnotations(entries[1]);
  expect(rowsOf(entries[1])).toEqual(['add-comment', '1', '2']);

  // a notice instead of the lines gets a table for the thread
  entries[1].lfsState = 'binary';
  entries[1].element = createElementFromHTML('<div><div class="code-diff"><div class="tw-p-3">Binary</div></div></div>');
  comments.renderAnnotations(entries[1]);
  expect(rowsOf(entries[1])).toEqual(['add-comment']);
  expect(entries[1].element.querySelector('.code-diff > table .diff-conversation')).not.toBeNull();
});
