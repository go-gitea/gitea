import {parseHunks} from './parse.ts';
import {CsvParseError, createCsvDiff, determineDelimiter, parseCsv} from './csvdiff.ts';

const cases = [
  {diff: `@@ -0,0 +1,2 @@
+col1,col2
+a,a`, base: '', head: `col1,col2
a,a`, cells: [[3, 3], [3, 3]]},
  {diff: `@@ -1,2 +1,3 @@
 col1,col2
-a,a
+a,a
+b,b`, base: `col1,col2
a,a`, head: `col1,col2
a,a
b,b`, cells: [[1, 1], [1, 1], [3, 3]]},
  {diff: `@@ -1,3 +1,2 @@
 col1,col2
-a,a
 b,b`, base: `col1,col2
a,a
b,b`, head: `col1,col2
b,b`, cells: [[1, 1], [4, 4], [1, 1]]},
  {diff: `@@ -1,2 +1,2 @@
 col1,col2
-b,b
+b,c`, base: `col1,col2
b,b`, head: `col1,col2
b,c`, cells: [[1, 1], [1, 2]]},
  {diff: `@@ -1,2 +0,0 @@
-col1,col2
-b,c`, base: `col1,col2
b,c`, head: '', cells: [[4, 4], [4, 4]]},
  {diff: `@@ -1,3 +1,3 @@
-col1,col2,col3
+cola,col2,col3
 a,b,c`, base: `col1,col2,col3
a,b,c`, head: `cola,col2,col3
a,b,c`, cells: [[4, 3, 1, 1], [4, 3, 1, 1]]},
  {diff: `@@ -1,2 +1,2 @@
-col1,col2,col3
-a,b,c
+col1,col1a,col2
+a,d,b`, base: `col1,col2,col3
a,b,c`, head: `col1,col1a,col2
a,d,b`, cells: [[1, 3, 4, 5], [1, 3, 4, 5]]},
  {diff: `@@ -1,2 +1,2 @@
-col1,col2,col3
-a,b,c
+col2,col3,col4
+b,c,d`, base: `col1,col2,col3
a,b,c`, head: `col2,col3,col4
b,c,d`, cells: [[4, 1, 1, 3], [4, 1, 1, 3]]},
  {diff: `@@ -1,2 +1,2 @@
-col1,col2,col
-a,b,c
+col3,col4,col5
+c,d,e`, base: `col1,col2,col3
a,b,c`, head: `col3,col4,col5
c,d,e`, cells: [[4, 5, 4, 3, 3], [4, 5, 4, 3, 3]]},
];

test('createCsvDiff', () => {
  for (const [idx, c] of cases.entries()) {
    const rows = (text: string) => (text ? parseCsv(text, determineDelimiter('unittest.csv', text)) : null);
    const sections = createCsvDiff(rows(c.base), rows(c.head), parseHunks(c.diff));
    expect(sections.flatMap((section) => section.map((row) => row.cells.map((cell) => cell?.type ?? 0))), `case ${idx}`).toEqual(c.cells);
  }
});

test('parseCsv', () => {
  expect(parseCsv('a,"b ""c"""\r\n\n d,e\n', ',')).toEqual([['a', 'b "c"'], ['d', 'e']]);
  expect(parseCsv('a\tb\n\tc', '\t')).toEqual([['a', 'b'], ['', 'c']]);
  expect(parseCsv('a,"multi\nline"\nb,c', ',')).toEqual([['a', 'multi\nline'], ['b', 'c']]);
  expect(() => parseCsv('a,b"c', ',')).toThrow(CsvParseError);
  expect(() => parseCsv('a,b\nc', ',', {sameFieldCount: true})).toThrow(CsvParseError);
});

test('determineDelimiter', () => {
  expect(determineDelimiter('a.csv', 'a;b;c\n1;2;3')).toEqual(';');
  expect(determineDelimiter('a.csv', 'a,b\n1,2')).toEqual(',');
  expect(determineDelimiter('a.tsv', 'a,b')).toEqual('\t');
  expect(determineDelimiter('a.csv', '"a b"|c')).toEqual('|');
});
