import {buildHunks} from './linediff.ts';

test('buildHunks', () => {
  const oldLines = Array.from({length: 20}, (_, i) => `line ${i + 1}`);
  const newLines = [...oldLines];
  newLines[1] = 'changed 2';
  newLines.splice(15, 1);
  const hunks = buildHunks(oldLines, newLines);
  expect(hunks.map((hunk) => hunk.header)).toEqual(['@@ -1,5 +1,5 @@', '@@ -13,7 +13,6 @@']);
  expect(hunks[1].lines.map((line) => `${line.type}:${line.oldNum}:${line.newNum}`)).toEqual([
    'same:13:13', 'same:14:14', 'same:15:15', 'del:16:0', 'same:17:16', 'same:18:17', 'same:19:18',
  ]);

  expect(buildHunks([], ['a', 'b']).map((hunk) => hunk.header)).toEqual(['@@ -0,0 +1,2 @@']);
  expect(buildHunks(['a'], []).map((hunk) => hunk.header)).toEqual(['@@ -1,1 +0,0 @@']);
  expect(buildHunks(['a'], ['a'])).toEqual([]);
});
