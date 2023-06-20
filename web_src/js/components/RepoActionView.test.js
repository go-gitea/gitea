import {expect, test} from 'vitest';
import {ansiLogToHTML} from './RepoActionView.vue';
import AnsiUp from 'ansi_up';

const ansi_up = new AnsiUp();

test('processConsoleLine', () => {
  expect(ansiLogToHTML('abc')).toEqual('abc');
  expect(ansiLogToHTML('abc\n')).toEqual('abc');
  expect(ansiLogToHTML('abc\r\n')).toEqual('abc');
  expect(ansiLogToHTML('\r')).toEqual('');
  expect(ansiLogToHTML('\rx\rabc')).toEqual('x\nabc');
  expect(ansiLogToHTML('\rabc\rx\r')).toEqual('abc\nx');

  expect(ansiLogToHTML('\x1b[30mblack\x1b[37mwhite')).toEqual(
    '<span style="color:rgb(0,0,0)">black</span><span style="color:rgb(255,255,255)">white</span>'
  );
  expect(ansiLogToHTML('<script>')).toEqual(
    '<span style="color:rgb(255,255,255)">&lt;script&gt;</span>'
  );

  expect(ansi_up.ansi_to_html('\x1b[1A\x1b[2Ktest\x1b[1B\x1b[1A\x1b[2K')).toEqual(
    'test'
  );
  expect(ansi_up.ansi_to_html('\x1b[1A\x1b[2K\rtest\r\x1b[1B\x1b[1A\x1b[2K')).toEqual(
    '\rtest\r'
  );

  expect(ansiLogToHTML('\x1b[1A\x1b[2Ktest\x1b[1B\x1b[1A\x1b[2K')).toEqual(
    '<span style="color:rgb(255,255,255)">test</span>',
  );
  expect(ansiLogToHTML('\x1b[1A\x1b[2K\rtest\r\x1b[1B\x1b[1A\x1b[2K')).toEqual(
    '<span style="color:rgb(255,255,255)">test</span>'
  );

  // treat "\033[0K" and "\033[0J" (Erase display/line) as "\r", then it will be covered to "\n" finally.
  expect(ansiLogToHTML('a\x1b[Kb\x1b[2Jc')).toEqual(
    '<span style="color:rgb(255,255,255)">a</span><span style="color:rgb(255,255,255)">b</span><span style="color:rgb(255,255,255)">c</span>'
  );
});
