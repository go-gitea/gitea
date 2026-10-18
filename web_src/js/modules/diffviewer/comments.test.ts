import {DiffComments, type CommentsHost} from './comments.ts';
import {DiffFile} from './entry.ts';
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
    ['new.txt', [comment(1, 'new.txt', 'new', 4), comment(2, 'new.txt', 'old', 4), comment(3, 'new.txt', 'new', 4), comment(4, 'new.txt', 'new', 1), comment(5, 'new.txt', 'new', 99)]],
    ['old.txt', [comment(6, 'old.txt', 'old', 3)]],
  ]);
  const threads = [...comments.threadsOf(entry).values()].map(({target, comments}) => `${target.side}:${target.line}=${comments.map((c) => c.id).join(',')}`);
  // both sides of a line are separate threads, the hidden line 1 waits for its gap, line 99 is outside the diff, the old name counts
  expect(threads).toEqual(['new:4=1,3', 'old:4=2', 'new:0=5', 'old:3=6']);
});
