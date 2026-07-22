import {cutString} from './string.ts';

test('cutString', () => {
  let [before, after, ok] = cutString('a = b = c', '=');
  expect(before).toBe('a ');
  expect(after).toBe(' b = c');
  expect(ok).toBe(true);

  [before, after, ok] = cutString(' a ', '=');
  expect(before).toBe(' a ');
  expect(after).toBe('');
  expect(ok).toBe(false);
});
