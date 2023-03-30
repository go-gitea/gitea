import {expect, test} from 'vitest';

import {processConsoleLine} from './RepoActionView.vue';

test('processConsoleLine', () => {
  expect(processConsoleLine('abc')).toEqual('abc');
  expect(processConsoleLine('abc\n')).toEqual('abc');
  expect(processConsoleLine('abc\r\n')).toEqual('abc');
  expect(processConsoleLine('\r')).toEqual('');
  expect(processConsoleLine('\rx\rabc')).toEqual('abc');
  expect(processConsoleLine('\rabc\rx\r')).toEqual('xbc');
});
