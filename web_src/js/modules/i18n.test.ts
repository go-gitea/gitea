import {trN} from './i18n.ts';

test('trN', () => {
  const originalLang = document.documentElement.lang;
  document.documentElement.lang = 'en-US';
  expect(trN(0, '%d job', '%d jobs')).toEqual('0 jobs');
  expect(trN(1, '%d job', '%d jobs')).toEqual('1 job');
  expect(trN(2, '%d job', '%d jobs')).toEqual('2 jobs');
  expect(trN(1000, '%d job', '%d jobs')).toEqual('1,000 jobs');
  // languages without a distinct singular always use the plural form
  document.documentElement.lang = 'zh-CN';
  expect(trN(1, '%d job', '%d jobs')).toEqual('1 jobs');
  document.documentElement.lang = originalLang;
});
