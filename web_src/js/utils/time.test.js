import {test, expect} from 'vitest';
import {formatTrackedTime} from './time.js';

test('formatTrackedTime', () => {
  expect(formatTrackedTime('')).toEqual('');
  expect(formatTrackedTime('0')).toEqual('');
  expect(formatTrackedTime('66')).toEqual('1 minute 6 seconds');
  expect(formatTrackedTime('52410')).toEqual('14 hours 33 minutes');
  expect(formatTrackedTime('563418')).toEqual('156 hours 30 minutes');
  expect(formatTrackedTime('1563418')).toEqual('434 hours 16 minutes');
  expect(formatTrackedTime('3937125')).toEqual('1093 hours 38 minutes');
  expect(formatTrackedTime('45677465')).toEqual('12688 hours 11 minutes');
});
