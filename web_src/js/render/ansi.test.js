import {renderAnsi} from './ansi.js';

test('renderAnsi', () => {
  expect(renderAnsi('abc')).toEqual('abc');
  expect(renderAnsi('abc\n')).toEqual('abc');
  expect(renderAnsi('abc\r\n')).toEqual('abc');
  expect(renderAnsi('\r')).toEqual('');
  expect(renderAnsi('\rx\rabc')).toEqual('x\nabc');
  expect(renderAnsi('\rabc\rx\r')).toEqual('abc\nx');
  expect(renderAnsi('\x1b[30mblack\x1b[37mwhite')).toEqual('<span class="ansi-black-fg">black</span><span class="ansi-white-fg">white</span>'); // unclosed
  expect(renderAnsi('<script>')).toEqual('&lt;script&gt;');
  expect(renderAnsi('\x1b[1A\x1b[2Ktest\x1b[1B\x1b[1A\x1b[2K')).toEqual('test');
  expect(renderAnsi('\x1b[1A\x1b[2K\rtest\r\x1b[1B\x1b[1A\x1b[2K')).toEqual('test');
  expect(renderAnsi('\x1b[1A\x1b[2Ktest\x1b[1B\x1b[1A\x1b[2K')).toEqual('test');
  expect(renderAnsi('\x1b[1A\x1b[2K\rtest\r\x1b[1B\x1b[1A\x1b[2K')).toEqual('test');

  // treat "\033[0K" and "\033[0J" (Erase display/line) as "\r", then it will be covered to "\n" finally.
  expect(renderAnsi('a\x1b[Kb\x1b[2Jc')).toEqual('a\nb\nc');
  expect(renderAnsi('\x1b[48;5;88ma\x1b[38;208;48;5;159mb\x1b[m')).toEqual(`<span style="background-color:rgb(135,0,0)">a</span><span style="background-color:rgb(175,255,255)">b</span>`);
});
