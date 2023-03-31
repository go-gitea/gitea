import {expect, test} from 'vitest';

import {ansiLogToHTML} from './RepoActionView.vue';

test('processConsoleLine', () => {
  expect(ansiLogToHTML('abc')).toEqual('abc');
  expect(ansiLogToHTML('abc\n')).toEqual('abc');
  expect(ansiLogToHTML('abc\r\n')).toEqual('abc');
  expect(ansiLogToHTML('\r')).toEqual('');
  expect(ansiLogToHTML('\rx\rabc')).toEqual('x\nabc');
  expect(ansiLogToHTML('\rabc\rx\r')).toEqual('abc\nx');

  expect(ansiLogToHTML('\x1b[30mblack\x1b[37mwhite')).toEqual('<span style="color:#000">black<span style="color:#AAA">white</span></span>');
  expect(ansiLogToHTML('<script>')).toEqual('&lt;script&gt;');
});
