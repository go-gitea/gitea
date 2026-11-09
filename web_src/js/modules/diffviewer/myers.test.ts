import {diffSequences, type DiffOp} from './myers.ts';

// applies the ops to "a" and checks that "b" is produced, returns a compact notation
function check(a: string, b: string): string {
  const ops = diffSequences(Array.from(a), Array.from(b));
  let aPos = 0;
  let bPos = 0;
  let result = '';
  for (const op of ops) {
    expect(op.aStart).toEqual(aPos);
    expect(op.bStart).toEqual(bPos);
    aPos = op.aEnd;
    bPos = op.bEnd;
    if (op.type === 'equal') {
      expect(a.slice(op.aStart, op.aEnd)).toEqual(b.slice(op.bStart, op.bEnd));
      result += a.slice(op.aStart, op.aEnd);
    } else if (op.type === 'delete') {
      result += `-[${a.slice(op.aStart, op.aEnd)}]`;
    } else {
      result += `+[${b.slice(op.bStart, op.bEnd)}]`;
    }
  }
  expect(aPos).toEqual(a.length);
  expect(bPos).toEqual(b.length);
  return result;
}

test('diffSequences', () => {
  expect(check('', '')).toEqual('');
  expect(check('abc', 'abc')).toEqual('abc');
  expect(check('', 'abc')).toEqual('+[abc]');
  expect(check('abc', '')).toEqual('-[abc]');
  expect(check('abc', 'axc')).toEqual('a-[b]+[x]c');
  expect(check('abcabba', 'cbabac')).toEqual('-[a]+[c]b-[c]ab-[b]a+[c]');
  expect(check('the quick fox', 'the slow fox')).toMatch(/^the .*fox$/);
  expect(check('xyz', 'abc')).toEqual('-[xyz]+[abc]');
});

test('diffSequences of all short strings', () => {
  const strings = [''];
  for (let i = 0; strings[i].length < 4; i++) strings.push(`${strings[i]}a`, `${strings[i]}b`);
  for (const a of strings) {
    for (const b of strings) check(a, b);
  }
});

test('diffSequences is minimal', () => {
  const editCount = (ops: DiffOp[]) => ops.reduce((sum, op) => sum + (op.type === 'equal' ? 0 : op.aEnd - op.aStart + op.bEnd - op.bStart), 0);
  // the LCS of these is "BCBA" or "BABA" etc. with length 4, so 3 + 2 = 5 edits
  expect(editCount(diffSequences(Array.from('ABCBDAB'), Array.from('BDCABA')))).toEqual(5);
});
