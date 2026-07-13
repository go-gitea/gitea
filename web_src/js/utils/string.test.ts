import {trPrintf} from './string.ts';

test('trPrintf', () => {
  expect(trPrintf('from %s to %d', 'main', 12)).toBe('from main to 12');
  expect(trPrintf('from %[1]s to %[2]d', 'main', 12)).toBe('from main to 12');
  expect(trPrintf('from %s to %[2]d', 'main', 12)).toBe('from main to 12');
  expect(trPrintf('from %s to %[3]d', 'main', 12)).toBe('from main to %[3]d');
});
