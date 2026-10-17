import {DiffComments} from './comments.ts';
import {parseHunks} from './parse.ts';
import {gapOfLine, gapsOf} from './expand.ts';
import type {DiffFileEntry} from './types.ts';

test('threadsOf', () => {
  // a changed line 5 of a renamed file, lines 1-2 are hidden above the hunk
  const entry = {file: {name: 'new.txt', oldName: 'old.txt'}, hunks: parseHunks('@@ -3,3 +3,3 @@\n a\n-b\n+B\n c\n')} as DiffFileEntry;
  const rowIndex = {old: new Map([[3, 1], [4, 2], [5, 4]]), new: new Map([[3, 1], [4, 3], [5, 4]])};
  const host = {
    config: {commentsUrl: '/comments'},
    findEntry: () => entry,
    hiddenGap: (_entry: DiffFileEntry, side: 'old' | 'new', line: number) => gapOfLine(gapsOf(entry.hunks, null), side, line),
    rowIndex: (_entry: DiffFileEntry, side: 'old' | 'new', line: number) => rowIndex[side].get(line) ?? -1,
  };
  const comments = new DiffComments(host as any);
  const comment = (id: number, path: string, side: 'old' | 'new', line: number) => ({id, path, side, line} as any);
  comments.byPath = new Map([
    ['new.txt', [comment(1, 'new.txt', 'new', 4), comment(2, 'new.txt', 'old', 4), comment(3, 'new.txt', 'new', 4), comment(4, 'new.txt', 'new', 1), comment(5, 'new.txt', 'new', 99)]],
    ['old.txt', [comment(6, 'old.txt', 'old', 3)]],
  ]);
  const threads = [...comments.threadsOf(entry).values()].map(({target, comments}) => `${target.side}:${target.line}=${comments.map((c) => c.id).join(',')}`);
  // both sides of a line are separate threads, the hidden line 1 waits for its gap, line 99 is outside the diff, the old name counts
  expect(threads).toEqual(['new:4=1,3', 'old:4=2', 'new:0=5', 'old:3=6']);
});
