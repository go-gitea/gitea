import {initTextareaMarkdown, markdownHandleIndention, textareaSplitLines} from './EditorMarkdown.ts';

test('textareaSplitLines', () => {
  let ret = textareaSplitLines('a\nbc\nd', 0);
  expect(ret).toEqual({lines: ['a', 'bc', 'd'], lengthBeforePosLine: 0, posLineIndex: 0, inlinePos: 0});

  ret = textareaSplitLines('a\nbc\nd', 1);
  expect(ret).toEqual({lines: ['a', 'bc', 'd'], lengthBeforePosLine: 0, posLineIndex: 0, inlinePos: 1});

  ret = textareaSplitLines('a\nbc\nd', 2);
  expect(ret).toEqual({lines: ['a', 'bc', 'd'], lengthBeforePosLine: 2, posLineIndex: 1, inlinePos: 0});

  ret = textareaSplitLines('a\nbc\nd', 3);
  expect(ret).toEqual({lines: ['a', 'bc', 'd'], lengthBeforePosLine: 2, posLineIndex: 1, inlinePos: 1});

  ret = textareaSplitLines('a\nbc\nd', 4);
  expect(ret).toEqual({lines: ['a', 'bc', 'd'], lengthBeforePosLine: 2, posLineIndex: 1, inlinePos: 2});

  ret = textareaSplitLines('a\nbc\nd', 5);
  expect(ret).toEqual({lines: ['a', 'bc', 'd'], lengthBeforePosLine: 5, posLineIndex: 2, inlinePos: 0});

  ret = textareaSplitLines('a\nbc\nd', 6);
  expect(ret).toEqual({lines: ['a', 'bc', 'd'], lengthBeforePosLine: 5, posLineIndex: 2, inlinePos: 1});
});

test('markdownHandleIndention', () => {
  const testInput = (input: string, expected?: string) => {
    const inputPos = input.indexOf('|');
    input = input.replace('|', '');
    const ret = markdownHandleIndention({value: input, selStart: inputPos, selEnd: inputPos});
    if (expected === null) {
      expect(ret).toEqual({handled: false});
    } else {
      const expectedPos = expected.indexOf('|');
      expected = expected.replace('|', '');
      expect(ret).toEqual({
        handled: true,
        valueSelection: {value: expected, selStart: expectedPos, selEnd: expectedPos},
      });
    }
  };

  testInput(`
  a|b
`, `
  a
  |b
`);

  testInput(`
1. a
2. |
`, `
1. a
|
`);

  testInput(`
|1. a
`, null); // let browser handle it

  testInput(`
1. a
1. b|c
`, `
1. a
2. b
3. |c
`);

  testInput(`
2. a
2. b|

1. x
1. y
`, `
1. a
2. b
3. |

1. x
1. y
`);

  testInput(`
2. a
2. b

1. x|
1. y
`, `
2. a
2. b

1. x
2. |
3. y
`);

  testInput(`
1. a
2. b|
3. c
`, `
1. a
2. b
3. |
4. c
`);

  testInput(`
1. a
  1. b
  2. b
  3. b
  4. b
1. c|
`, `
1. a
  1. b
  2. b
  3. b
  4. b
2. c
3. |
`);

  testInput(`
1. a
2. a
3. a
4. a
5. a
6. a
7. a
8. a
9. b|c
`, `
1. a
2. a
3. a
4. a
5. a
6. a
7. a
8. a
9. b
10. |c
`);

  // this is a special case, it's difficult to re-format the parent level at the moment, so leave it to the future
  testInput(`
1. a
  2. b|
3. c
`, `
1. a
  1. b
  2. |
3. c
`);
});

test('EditorMarkdown', () => {
  const textarea = document.createElement('textarea');
  initTextareaMarkdown(textarea);

  type ValueWithCursor = string | {
    value: string;
    pos: number;
  }
  const testInput = (input: ValueWithCursor, result: ValueWithCursor) => {
    const intputValue = typeof input === 'string' ? input : input.value;
    const inputPos = typeof input === 'string' ? intputValue.length : input.pos;
    textarea.value = intputValue;
    textarea.setSelectionRange(inputPos, inputPos);

    const e = new KeyboardEvent('keydown', {key: 'Enter', cancelable: true});
    textarea.dispatchEvent(e);
    if (!e.defaultPrevented) textarea.value += '\n'; // simulate default behavior

    const expectedValue = typeof result === 'string' ? result : result.value;
    const expectedPos = typeof result === 'string' ? expectedValue.length : result.pos;
    expect(textarea.value).toEqual(expectedValue);
    expect(textarea.selectionStart).toEqual(expectedPos);
  };

  testInput('-', '-\n');
  testInput('1.', '1.\n');

  testInput('- ', '');
  testInput('1. ', '');
  testInput({value: '1. \n2. ', pos: 3}, {value: '\n2. ', pos: 0});

  testInput('- x', '- x\n- ');
  testInput('1. foo', '1. foo\n2. ');
  testInput({value: '1. a\n2. b\n3. c', pos: 4}, {value: '1. a\n2. \n3. b\n4. c', pos: 8});
  testInput('- [ ]', '- [ ]\n- ');
  testInput('- [ ] foo', '- [ ] foo\n- [ ] ');
  testInput('* [x] foo', '* [x] foo\n* [ ] ');
  testInput('1. [x] foo', '1. [x] foo\n2. [ ] ');
});
