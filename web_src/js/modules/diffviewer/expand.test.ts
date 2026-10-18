import {parseHunks} from './parse.ts';
import {expandDirections, expandGap, gapOfLine, gapsOf} from './expand.ts';

const fileLines = Array.from({length: 100}, (_, i) => `line ${i + 1}`);

// a modified line 30 and a modified line 60 of a 100 line file
const patch = `@@ -27,7 +27,7 @@ func a() {
 line 27
 line 28
 line 29
-old 30
+line 30
 line 31
 line 32
 line 33
@@ -57,7 +57,7 @@
 line 57
 line 58
 line 59
-old 60
+line 60
 line 61
 line 62
 line 63
`;

test('gapsOf', () => {
  const hunks = parseHunks(patch);
  expect(gapsOf(hunks, null)).toEqual([
    {index: 0, newFrom: 1, newTo: 26, oldFrom: 1, oldTo: 26},
    {index: 1, newFrom: 34, newTo: 56, oldFrom: 34, oldTo: 56},
    {index: 2, newFrom: 64, newTo: Infinity, oldFrom: 64, oldTo: Infinity},
  ]);
  expect(gapsOf(hunks, 100).at(-1)).toEqual({index: 2, newFrom: 64, newTo: 100, oldFrom: 64, oldTo: 100});
  expect(gapOfLine(gapsOf(hunks, null), 'old', 40)?.index).toEqual(1);
  expect(gapOfLine(gapsOf(hunks, null), 'new', 30)).toBeNull();
});

test('expandGap', () => {
  const hunks = parseHunks(patch);
  const gaps = gapsOf(hunks, 100);
  expect(expandDirections(gaps[0], 2)).toEqual(['up']);
  expect(expandDirections(gaps[1], 2)).toEqual(['down', 'up']);
  expect(expandDirections(gaps[2], 2)).toEqual(['down']);

  expandGap(hunks, gaps[0], 'up', fileLines);
  expect(hunks[0].header).toEqual('@@ -7,27 +7,27 @@ func a() {');
  expect(hunks[0].lines[0]).toEqual({type: 'same', oldNum: 7, newNum: 7, content: 'line 7', expanded: true});

  expandGap(hunks, gapsOf(hunks, 100)[1], 'down', fileLines);
  expect(hunks[0].header).toEqual('@@ -7,47 +7,47 @@ func a() {');
  expect(gapsOf(hunks, 100)[1]).toEqual({index: 1, newFrom: 54, newTo: 56, oldFrom: 54, oldTo: 56});

  // the remaining small gap merges the hunks
  expandGap(hunks, gapsOf(hunks, 100)[1], 'all', fileLines);
  expect(hunks).toHaveLength(1);
  expect(hunks[0].header).toEqual('@@ -7,57 +7,57 @@ func a() {');

  expandGap(hunks, gapsOf(hunks, 100).at(-1)!, 'all', fileLines);
  expect(hunks[0].lines.at(-1)).toEqual({type: 'same', oldNum: 100, newNum: 100, content: 'line 100', expanded: true});
  expect(gapsOf(hunks, 100).map((gap) => gap.index)).toEqual([0]);
});

test('expandGap with insertion hunk', () => {
  // two lines inserted after line 5, so the old side of the hunk is empty
  const hunks = parseHunks('@@ -5,0 +6,2 @@\n+a\n+b\n');
  const lines = ['1', '2', '3', '4', '5', 'a', 'b', '8', '9'];
  // without context after its changes, the last hunk ends the file as far as the diff can tell
  expect(gapsOf(hunks, null).map((gap) => gap.index)).toEqual([0]);
  expect(gapsOf(hunks, 9)).toEqual([
    {index: 0, newFrom: 1, newTo: 5, oldFrom: 1, oldTo: 5},
    {index: 1, newFrom: 8, newTo: 9, oldFrom: 6, oldTo: 7},
  ]);
  expandGap(hunks, gapsOf(hunks, 9)[1], 'all', lines);
  expect(hunks[0].header).toEqual('@@ -6,2 +6,4 @@');
  expect(hunks[0].lines.at(-1)).toEqual({type: 'same', oldNum: 7, newNum: 9, content: '9', expanded: true});
  expandGap(hunks, gapsOf(hunks, 9)[0], 'all', lines);
  expect(hunks[0].header).toEqual('@@ -1,7 +1,9 @@');
});
