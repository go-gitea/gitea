import {initTextareaMarkdown} from './EditorMarkdown.ts';

test('EditorMarkdown', () => {
  const textarea = document.createElement('textarea');
  initTextareaMarkdown(textarea);

  const testInput = (value, expected) => {
    textarea.value = value;
    textarea.setSelectionRange(value.length, value.length);
    const e = new KeyboardEvent('keydown', {key: 'Enter', cancelable: true});
    textarea.dispatchEvent(e);
    if (!e.defaultPrevented) textarea.value += '\n';
    expect(textarea.value).toEqual(expected);
  };

  testInput('-', '-\n');
  testInput('1.', '1.\n');

  testInput('- ', '');
  testInput('1. ', '');

  testInput('- x', '- x\n- ');
  testInput('- [ ]', '- [ ]\n- ');
  testInput('- [ ] foo', '- [ ] foo\n- [ ] ');
  testInput('* [x] foo', '* [x] foo\n* [ ] ');
  testInput('1. [x] foo', '1. [x] foo\n1. [ ] ');
});
