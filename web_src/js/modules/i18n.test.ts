import {trN, trString} from './i18n.ts';

test('trN', () => {
  expect(trN(0, '%d job', '%d jobs', {lang: 'en-US'})).toEqual('0 jobs');
  expect(trN(1, '%d job', '%d jobs', {lang: 'en-US'})).toEqual('1 job');
  expect(trN(2, '%d job', '%d jobs', {lang: 'en-US'})).toEqual('2 jobs');
  expect(trN(1000, '%d job', '%d jobs', {lang: 'en-US'})).toEqual('1000 jobs');
  // languages without a distinct singular always use the plural form
  expect(trN(1, '%d job', '%d jobs', {lang: 'zh-CN'})).toEqual('1 jobs');
});

test('trString', () => {
  expect(trString('from %s to %d', 'main', 12)).toBe('from main to 12');
  expect(trString('from %[1]s to %[2]d', 'main', 12)).toBe('from main to 12');
  expect(trString('from %s to %[2]d', 'main', 12)).toBe('from main to 12');
  expect(trString('from %s to %[3]d', 'main', 12)).toBe('from main to %[3]d');
});
