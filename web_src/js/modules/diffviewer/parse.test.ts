import {parseHunks} from './parse.ts';

test('parseHunks', () => {
  const hunks = parseHunks(`@@ -1,3 +1,3 @@ function foo() {
 const a = 1;
-const b = 2;
+const b = 3;
 const c = 3;
\\ No newline at end of file
@@ -10 +10,2 @@
 x
+y
`);
  expect(hunks).toEqual([
    {header: '@@ -1,3 +1,3 @@ function foo() {', oldStart: 1, oldCount: 3, newStart: 1, newCount: 3, lines: [
      {type: 'same', oldNum: 1, newNum: 1, content: 'const a = 1;'},
      {type: 'del', oldNum: 2, newNum: 0, content: 'const b = 2;'},
      {type: 'add', oldNum: 0, newNum: 2, content: 'const b = 3;'},
      {type: 'same', oldNum: 3, newNum: 3, content: 'const c = 3;'},
    ]},
    {header: '@@ -10 +10,2 @@', oldStart: 10, oldCount: 1, newStart: 10, newCount: 2, lines: [
      {type: 'same', oldNum: 10, newNum: 10, content: 'x'},
      {type: 'add', oldNum: 0, newNum: 11, content: 'y'},
    ]},
  ]);
  expect(parseHunks('')).toEqual([]);
});
