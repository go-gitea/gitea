import {trN} from './i18n.ts';
import {getCurrentLocale} from '../utils.ts';

vi.mock('../utils.ts', () => ({getCurrentLocale: vi.fn()}));

test('trN', () => {
  vi.mocked(getCurrentLocale).mockReturnValue('en-US');
  expect(trN(0, '%d job', '%d jobs')).toEqual('0 jobs');
  expect(trN(1, '%d job', '%d jobs')).toEqual('1 job');
  expect(trN(2, '%d job', '%d jobs')).toEqual('2 jobs');
  expect(trN(1000, '%d job', '%d jobs')).toEqual('1000 jobs');
  // languages without a distinct singular always use the plural form
  vi.mocked(getCurrentLocale).mockReturnValue('zh-CN');
  expect(trN(1, '%d job', '%d jobs')).toEqual('1 jobs');
});
