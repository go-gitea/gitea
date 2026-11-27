import {DiffFile} from './entry.ts';
import {expandGap} from './expand.ts';
import {parseHunks} from './parse.ts';
import type {DiffStreamFile} from './stream.ts';

test('DiffFile', () => {
  const file: DiffStreamFile = {name: 'a.txt', oldName: 'a.txt', nameHash: 'x', type: 'modified', tabSize: 4};
  // rows: header, a, -b, +B, c
  const entry = new DiffFile(file, parseHunks('@@ -3,3 +3,3 @@\n a\n-b\n+B\n c\n'), false, false);
  expect(entry.changeCounts()).toEqual({additions: 1, deletions: 1});
  expect(entry.rowIndex('old', 4)).toBe(2);
  expect(entry.rowIndex('new', 4)).toBe(3);
  expect(entry.rowIndex('new', 99)).toBe(-1);
  expect(entry.rowCount(false)).toBe(5);
  expect(entry.gaps(false)).toEqual([]);

  // the lines above the hunk are a gap, the tail is unknown until the content is loaded
  entry.invalidateRows();
  expect(entry.gaps(true).map((gap) => [gap.newFrom, gap.newTo])).toEqual([[1, 2]]);
  entry.setContent({old: null, new: Array.from({length: 10}, (_, i) => `line ${i + 1}`)});
  expect(entry.gaps(true).map((gap) => [gap.newFrom, gap.newTo])).toEqual([[1, 2], [6, 10]]);

  // expanding changes the hunks in place
  expandGap(entry.hunks, entry.gaps(true)[0], 'all', entry.content!.new);
  entry.invalidateRows();
  expect(entry.rowIndex('new', 1)).toBe(1);
  expect(entry.rowCount(false)).toBe(7);
  expect(entry.gaps(true).map((gap) => [gap.newFrom, gap.newTo])).toEqual([[6, 10]]);

  const split = new DiffFile(file, parseHunks('@@ -1,2 +1,2 @@\n-a\n+A\n-b\n'), false, false);
  expect(split.rowCount(true)).toBe(3); // header, a/A, b
});
