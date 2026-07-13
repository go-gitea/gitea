import {trPrintf} from './string.ts';

test('sprintf supports non-indexed placeholders', () => {
  expect(trPrintf('from %s to %d', 'main', 12)).toBe('from main to 12');
});

test('sprintf supports indexed placeholders', () => {
  expect(trPrintf('from %[1]s to %[2]d', 'main', 12)).toBe('from main to 12');
});

test('sprintf supports mixed indexed placeholders', () => {
  expect(trPrintf('from %s to %[2]d', 'main', 12)).toBe('from main to 12');
});

test('sprintf keeps unsupported placeholders unchanged', () => {
  expect(trPrintf('from %s to %[3]d', 'main', 12)).toBe('from main to %[3]d');
});
