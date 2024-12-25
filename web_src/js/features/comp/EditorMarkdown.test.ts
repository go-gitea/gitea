import {initTextareaMarkdown} from './EditorMarkdown.ts';

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
  testInput('1. foo', '1. foo\n1. ');
  testInput({value: '1. a\n2. b\n3. c', pos: 4}, {value: '1. a\n1. \n2. b\n3. c', pos: 8});
  testInput('- [ ]', '- [ ]\n- ');
  testInput('- [ ] foo', '- [ ] foo\n- [ ] ');
  testInput('* [x] foo', '* [x] foo\n* [ ] ');
  testInput('1. [x] foo', '1. [x] foo\n1. [ ] ');
});
