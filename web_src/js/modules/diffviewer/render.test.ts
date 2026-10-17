import {parseHunks} from './parse.ts';
import {pairChangedLines, renderSplitRows, renderUnifiedRows, segmentsToHtml} from './render.ts';
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

test('renderRows', () => {
  const hunks = parseHunks('@@ -1,2 +1,2 @@\n a\n-b\n+B\n');
  hunks[0].lines.unshift({type: 'same', oldNum: 0, newNum: 0, content: '', expanded: true}); // like an expanded context line
  const lineHtml = ['', 'a', 'b', 'B'];
  const opts = {addCommentLabel: 'add', escapeButtons: ['', '', '', ''], expanders: new Map([[0, '<i>up</i>']])};
  const rows = (html: string) => Array.from(new DOMParser().parseFromString(`<table>${html}</table>`, 'text/html').querySelectorAll('tr'));
  const unified = rows(renderUnifiedRows(hunks, lineHtml, opts));
  expect(unified.map((tr) => `${tr.getAttribute('data-line-type')}:${tr.getAttribute('data-old')}:${tr.getAttribute('data-new')}`)).toEqual(['tag:null:null', 'same::', 'same:1:1', 'del:2:', 'add::2']);
  expect(unified[0].querySelector('.code-expander-buttons, i')?.textContent).toEqual('up');
  expect(unified[3].querySelector('.add-code-comment')?.getAttribute('data-side')).toEqual('old');
  // the split view pairs the changed lines into one row, expanded lines are only commented on the new side
  const split = rows(renderSplitRows(hunks, lineHtml, pairChangedLines(hunks), opts));
  expect(split.map((tr) => `${tr.getAttribute('data-line-type')}:${tr.getAttribute('data-old')}:${tr.getAttribute('data-new')}`)).toEqual(['tag:null:null', 'same::', 'same:1:1', 'del:2:2']);
  expect(split[1].querySelectorAll('.add-code-comment[data-side="old"]')).toHaveLength(0);
  expect(split[3].querySelectorAll('.add-code-comment')).toHaveLength(2);
  expect(split[3].querySelector('.lines-code-new .code-inner')?.textContent).toEqual('B');
});

test('wordDiffRanges', () => {
  expect(wordDiffRanges('const foo = 1;', 'const bar = 1;')).toEqual({old: [[6, 9]], new: [[6, 9]]});
  expect(wordDiffRanges('a', 'a b')).toEqual({old: [], new: [[1, 3]]});
  expect(wordDiffRanges('x'.repeat(10), 'y')).toEqual({old: [[0, 10]], new: [[0, 1]]});
});
