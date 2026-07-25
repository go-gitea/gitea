import {AnsiLineRenderer, renderAnsiInto} from './ansi.ts';

const renderAnsi = (line: string) => {
  const el = document.createElement('div');
  renderAnsiInto(el, line);
  return el.innerHTML;
};

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

  // treat "\033[0K" and "\033[0J" (Erase display/line) as "\r", then it will be covered to "\n" finally.
  expect(renderAnsi('a\x1b[Kb\x1b[2Jc')).toEqual('a\nb\nc');
  expect(renderAnsi('\x1b[48;5;88ma\x1b[38;208;48;5;159mb\x1b[m')).toEqual(`<span style="background-color:rgb(135,0,0)">a</span><span style="background-color:rgb(175,255,255)">b</span>`);

  // URLs in ANSI output become clickable links
  const link = (url: string) => `<a href="${url}" target="_blank">${url}</a>`;
  expect(renderAnsi('foo https://example.com bar')).toEqual(`foo ${link('https://example.com')} bar`);
  expect(renderAnsi('<https://example.com?a=b&c=d#h>')).toEqual(`&lt;${link('https://example.com?a=b&amp;c=d#h')}&gt;`);
  expect(renderAnsi('open https://example.com.')).toEqual(`open ${link('https://example.com')}.`);
  expect(renderAnsi('"https://example.com"')).toEqual(`"${link('https://example.com')}"`);
  expect(renderAnsi('\x1b[32mhttps://example.com\x1b[0m')).toEqual(`<span class="ansi-green-fg">${link('https://example.com')}</span>`);
});

test('renderAnsi colors and attributes', () => {
  // the 16 named colors use css classes, foreground and background
  expect(renderAnsi('\x1b[31mred')).toEqual('<span class="ansi-red-fg">red</span>');
  expect(renderAnsi('\x1b[41mred bg')).toEqual('<span class="ansi-red-bg">red bg</span>');
  expect(renderAnsi('\x1b[91mbright')).toEqual('<span class="ansi-bright-red-fg">bright</span>');
  expect(renderAnsi('\x1b[101mbright bg')).toEqual('<span class="ansi-bright-red-bg">bright bg</span>');

  // text attributes and their individual resets
  expect(renderAnsi('\x1b[1mbold')).toEqual('<span style="font-weight:bold">bold</span>');
  expect(renderAnsi('\x1b[2mfaint')).toEqual('<span style="opacity:0.7">faint</span>');
  expect(renderAnsi('\x1b[3mitalic')).toEqual('<span style="font-style:italic">italic</span>');
  expect(renderAnsi('\x1b[4munderline')).toEqual('<span style="text-decoration:underline">underline</span>');
  expect(renderAnsi('\x1b[1;4;31mall')).toEqual('<span style="font-weight:bold;text-decoration:underline" class="ansi-red-fg">all</span>');
  expect(renderAnsi('\x1b[1mb\x1b[22mplain')).toEqual('<span style="font-weight:bold">b</span>plain');
  expect(renderAnsi('\x1b[31mr\x1b[39mplain')).toEqual('<span class="ansi-red-fg">r</span>plain');

  // 256-color palette: the first 16 map onto the css classes, the rest onto inline colors
  expect(renderAnsi('\x1b[38;5;1mx')).toEqual('<span class="ansi-red-fg">x</span>');
  expect(renderAnsi('\x1b[38;5;9mx')).toEqual('<span class="ansi-bright-red-fg">x</span>');
  expect(renderAnsi('\x1b[38;5;16mx')).toEqual('<span style="color:rgb(0,0,0)">x</span>');
  expect(renderAnsi('\x1b[38;5;231mx')).toEqual('<span style="color:rgb(255,255,255)">x</span>');
  expect(renderAnsi('\x1b[38;5;232mx')).toEqual('<span style="color:rgb(8,8,8)">x</span>');
  expect(renderAnsi('\x1b[38;5;255mx')).toEqual('<span style="color:rgb(238,238,238)">x</span>');
  expect(renderAnsi('\x1b[38;5;256mx')).toEqual('x'); // out of range, ignored

  // truecolor
  expect(renderAnsi('\x1b[38;2;1;2;3mx')).toEqual('<span style="color:rgb(1,2,3)">x</span>');
  expect(renderAnsi('\x1b[48;2;1;2;3mx')).toEqual('<span style="background-color:rgb(1,2,3)">x</span>');
  expect(renderAnsi('\x1b[38;2;300;0;0mx')).toEqual('x'); // out of range, ignored

  // colon sub-parameters: "4:0" turns the underline off, the other styles all render as underline
  expect(renderAnsi('\x1b[4:3mcurly')).toEqual('<span style="text-decoration:underline">curly</span>');
  expect(renderAnsi('\x1b[4:3mon\x1b[4:0moff')).toEqual('<span style="text-decoration:underline">on</span>off');

  // 58 sets an underline color that is not rendered, its parameters must not be read as codes
  expect(renderAnsi('\x1b[4m\x1b[58;2;135;0;255mstill underlined')).toEqual('<span style="text-decoration:underline">still underlined</span>');
  expect(renderAnsi('\x1b[58;5;9mno color')).toEqual('no color');

  // "\x1b[m" resets everything, same as "\x1b[0m"
  expect(renderAnsi('\x1b[1;31mx\x1b[my')).toEqual('<span style="font-weight:bold" class="ansi-red-fg">x</span>y');
});

test('renderAnsi drops sequences it does not render', () => {
  // non-SGR sequences are removed rather than shown as text
  expect(renderAnsi('a\x1b[?25lb')).toEqual('ab');
  expect(renderAnsi('a\x1b[1;2Hb')).toEqual('ab');
  expect(renderAnsi('a\x1b[6nb')).toEqual('ab');
  expect(renderAnsi('a\x1b]0;window title\x07b')).toEqual('ab');
  expect(renderAnsi('a\x1b]8;;http://example.com\x1b\\b')).toEqual('ab'); // unterminated hyperlink

  // a sequence cut off by the end of the line can never be completed, so it is dropped
  expect(renderAnsi('abc\x1b[3')).toEqual('abc');
  expect(renderAnsi('abc\x1b[')).toEqual('abc');
  expect(renderAnsi('abc\x1b')).toEqual('abc');
});

test('renderAnsi OSC 8 hyperlinks', () => {
  const esc = '\x1b';
  const link = (raw: string) => renderAnsi(raw);

  // both string terminators, "\x07" and "\x1b\\", are accepted
  expect(link(`${esc}]8;;https://example.com${esc}\\click here${esc}]8;;${esc}\\`)).toEqual('<a href="https://example.com" target="_blank">click here</a>');
  expect(link(`${esc}]8;;https://example.com\x07click here${esc}]8;;\x07`)).toEqual('<a href="https://example.com" target="_blank">click here</a>');
  expect(link(`before ${esc}]8;;https://example.com${esc}\\text${esc}]8;;${esc}\\ after`)).toEqual('before <a href="https://example.com" target="_blank">text</a> after');

  // the id parameter is ignored, the style in effect still applies to the label
  expect(link(`${esc}]8;id=1;https://example.com${esc}\\text${esc}]8;;${esc}\\`)).toEqual('<a href="https://example.com" target="_blank">text</a>');
  expect(link(`${esc}[31m${esc}]8;;https://example.com${esc}\\text${esc}]8;;${esc}\\`)).toEqual('<a href="https://example.com" target="_blank"><span class="ansi-red-fg">text</span></a>');

  // a non-web target is not linked, but its label is still shown
  expect(link(`${esc}]8;;javascript:alert(1)${esc}\\text${esc}]8;;${esc}\\`)).toEqual('text');
  expect(link(`${esc}]8;;ftp://example.com${esc}\\text${esc}]8;;${esc}\\`)).toEqual('text');

  // the url and the label are both escaped
  expect(link(`${esc}]8;;https://example.com/?a=1&b="x"${esc}\\<b>${esc}]8;;${esc}\\`)).toEqual('<a href="https://example.com/?a=1&amp;b=&quot;x&quot;" target="_blank">&lt;b&gt;</a>');
});

test('AnsiLineRenderer carries style across lines', () => {
  const ansi = new AnsiLineRenderer();
  const render = (line: string) => {
    const el = document.createElement('div');
    ansi.renderInto(el, line);
    return el.innerHTML;
  };

  // an unterminated color keeps applying to the following lines, including plain ones
  expect(render('\x1b[31mred')).toEqual('<span class="ansi-red-fg">red</span>');
  expect(render('still red')).toEqual('<span class="ansi-red-fg">still red</span>');
  expect(render('\x1b[1mbold too')).toEqual('<span style="font-weight:bold" class="ansi-red-fg">bold too</span>');

  // until it is reset, after which plain lines are plain again
  expect(render('\x1b[0m')).toEqual('');
  expect(render('plain')).toEqual('plain');

  // a truncated sequence is not carried into the next line
  expect(render('oops\x1b[3')).toEqual('oops');
  expect(render('unaffected')).toEqual('unaffected');

  // renderers are independent of each other
  const other = new AnsiLineRenderer();
  const otherEl = document.createElement('div');
  other.renderInto(otherEl, 'clean');
  expect(otherEl.innerHTML).toEqual('clean');
});
