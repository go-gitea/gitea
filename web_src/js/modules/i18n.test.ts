import {trPlural, trString} from './i18n.ts';

test('trPlural', () => {
  const en = {one: '%d job', other: '%d jobs'};
  expect(trPlural(0, en, {lang: 'en-US'})).toEqual('0 jobs');
  expect(trPlural(1, en, {lang: 'en-US'})).toEqual('1 job');
  expect(trPlural(2, en, {lang: 'en-US'})).toEqual('2 jobs');
  expect(trPlural(1000, en, {lang: 'en-US'})).toEqual('1000 jobs');
  // languages without a distinct singular always use the `other` form
  expect(trPlural(1, en, {lang: 'zh-CN'})).toEqual('1 jobs');

  const ru = {one: '%d задача', few: '%d задачи', many: '%d задач', other: '%d задачи'};
  const ruCases: Array<[number, string]> = [[1, '1 задача'], [2, '2 задачи'], [5, '5 задач'], [11, '11 задач'], [21, '21 задача'], [22, '22 задачи'], [25, '25 задач']];
  for (const [count, expected] of ruCases) expect(trPlural(count, ru, {lang: 'ru-RU'})).toEqual(expected);

  // a missing category falls back to `other`
  expect(trPlural(5, {other: '%d jobs'}, {lang: 'ru-RU'})).toEqual('5 jobs');
});

test('trString', () => {
  expect(trString('from %s to %d', 'main', 12)).toBe('from main to 12');
  expect(trString('from %[1]s to %[2]d', 'main', 12)).toBe('from main to 12');
  expect(trString('from %s to %[2]d', 'main', 12)).toBe('from main to 12');
  expect(trString('from %s to %[3]d', 'main', 12)).toBe('from main to %[3]d');
});
