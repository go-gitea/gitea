import {expect, test} from 'vitest';

import {ansiLogToHTML} from './RepoActionView.vue';
import AnsiToHTML from 'ansi-to-html';

test('processConsoleLine', () => {
  expect(ansiLogToHTML('abc')).toEqual('abc');
  expect(ansiLogToHTML('abc\n')).toEqual('abc');
  expect(ansiLogToHTML('abc\r\n')).toEqual('abc');
  expect(ansiLogToHTML('\r')).toEqual('');
  expect(ansiLogToHTML('\rx\rabc')).toEqual('x\nabc');
  expect(ansiLogToHTML('\rabc\rx\r')).toEqual('abc\nx');

  expect(ansiLogToHTML('\x1b[30mblack\x1b[37mwhite')).toEqual('<span style="color:#000">black<span style="color:#AAA">white</span></span>');
  expect(ansiLogToHTML('<script>')).toEqual('&lt;script&gt;');


  // upstream AnsiToHTML has bugs when processing "\033[1A" and "\033[1B", we fixed these control sequences in our code
  // if upstream could fix these bugs, we can remove these tests and remove our patch code
  const ath = new AnsiToHTML({escapeXML: true});
  expect(ath.toHtml('\x1b[1A\x1b[2Ktest\x1b[1B\x1b[1A\x1b[2K')).toEqual('AtestBA'); // AnsiToHTML bug
  expect(ath.toHtml('\x1b[1A\x1b[2K\rtest\r\x1b[1B\x1b[1A\x1b[2K')).toEqual('A\rtest\rBA'); // AnsiToHTML bug

  // test our patched behavior
  expect(ansiLogToHTML('\x1b[1A\x1b[2Ktest\x1b[1B\x1b[1A\x1b[2K')).toEqual('test');
  expect(ansiLogToHTML('\x1b[1A\x1b[2K\rtest\r\x1b[1B\x1b[1A\x1b[2K')).toEqual('test');

  // treat "\033[0K" and "\033[0J" (Erase display/line) as "\r", then it will be covered to "\n" finally.
  expect(ansiLogToHTML('a\x1b[Kb\x1b[2Jc')).toEqual('a\nb\nc');
});
