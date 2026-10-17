import {parseHunks} from './parse.ts';
import {pairChangedLines, segmentsToHtml} from './render.ts';
import {wordDiffRanges} from './worddiff.ts';

test('pairChangedLines', () => {
  const hunks = parseHunks(`@@ -1,4 +1,4 @@
 a
-b
-c
+B
 d
-e
@@ -10 +10 @@
+x
`);
  // lines: a b c B d e | x
  expect(Array.from(pairChangedLines(hunks))).toEqual([-1, 3, -1, 1, -1, -1, -1]);
});

test('segmentsToHtml', () => {
  expect(segmentsToHtml([['a<b', '']])).toEqual('a&lt;b');
  expect(segmentsToHtml([['const', 'tok-keyword'], [' x = 1', '']], [[4, 7]], 'added-code')).toEqual(
    '<span class="tok-keyword">cons</span><span class="added-code"><span class="tok-keyword">t</span></span><span class="added-code"> x</span> = 1',
  );
});

test('wordDiffRanges', () => {
  expect(wordDiffRanges('const foo = 1;', 'const bar = 1;')).toEqual({old: [[6, 9]], new: [[6, 9]]});
  expect(wordDiffRanges('a', 'a b')).toEqual({old: [], new: [[1, 3]]});
  expect(wordDiffRanges('x'.repeat(10), 'y')).toEqual({old: [[0, 10]], new: [[0, 1]]});
});
