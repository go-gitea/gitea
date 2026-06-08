import {insertSavedReply} from './SavedReplies.ts';

test('insertSavedReply into empty textarea', () => {
  const result = insertSavedReply('', 0, 'Hello world');
  expect(result).toEqual({value: 'Hello world', pos: 11});
});

test('insertSavedReply at end of single line', () => {
  const result = insertSavedReply('existing text', 13, 'reply');
  expect(result).toEqual({value: 'existing text\nreply', pos: 19});
});

test('insertSavedReply at end of current line (multiline)', () => {
  // cursor on first line, should insert after the first line
  const result = insertSavedReply('line1\nline2\nline3', 3, 'reply');
  expect(result).toEqual({value: 'line1\nreply\nline2\nline3', pos: 11});
});

test('insertSavedReply with cursor at start', () => {
  const result = insertSavedReply('line1\nline2', 0, 'reply');
  expect(result).toEqual({value: 'line1\nreply\nline2', pos: 11});
});
