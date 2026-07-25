import {AnsiLineRenderer} from './ansi.ts';

test('renderAnsi', () => {
  const renderAnsi = (line: string, ansi = new AnsiLineRenderer()) => {
    const el = document.createElement('div');
    ansi.renderLine(el, line);
    return el.innerHTML;
  };

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
  expect(renderAnsi('\x1b[48;5;88ma\x1b[38;208;48;5;159mb\x1b[m')).toEqual(`<span style="background-color:#870000">a</span><span style="background-color:#afffff">b</span>`);

  // URLs in ANSI output become clickable links
  const link = (url: string) => `<a href="${url}" target="_blank">${url}</a>`;
  expect(renderAnsi('foo https://example.com bar')).toEqual(`foo ${link('https://example.com')} bar`);
  expect(renderAnsi('<https://example.com?a=b&c=d#h>')).toEqual(`&lt;${link('https://example.com?a=b&amp;c=d#h')}&gt;`);
  expect(renderAnsi('open https://example.com.')).toEqual(`open ${link('https://example.com')}.`);
  expect(renderAnsi('"https://example.com"')).toEqual(`"${link('https://example.com')}"`);
  expect(renderAnsi('\x1b[32mhttps://example.com\x1b[0m')).toEqual(`<span class="ansi-green-fg">${link('https://example.com')}</span>`);

  // attributes, faint nesting so its color mixes with the outer one, conceal emitting no foreground
  expect(renderAnsi('\x1b[1;5;8mx')).toEqual('<span class="ansi-bold ansi-blink ansi-conceal">x</span>');
  expect(renderAnsi('\x1b[2;31mx')).toEqual('<span class="ansi-red-fg"><span class="ansi-faint">x</span></span>');
  expect(renderAnsi('\x1b[31;7mx')).toEqual('<span class="ansi-inverse-fg ansi-red-bg">x</span>');
  expect(renderAnsi('\x1b[4:3;9;53mx')).toEqual('<span class="ansi-underline ansi-line-through ansi-overline ansi-wavy">x</span>');
  expect(renderAnsi('\x1b[4;58;5;9mx')).toEqual('<span class="ansi-underline" style="text-decoration-color:var(--color-ansi-bright-red)">x</span>');

  // OSC 8 hyperlinks
  expect(renderAnsi('\x1b]8;;https://example.com\x1b\\text\x1b]8;;\x1b\\')).toEqual(`<a href="https://example.com" target="_blank">text</a>`);
  expect(renderAnsi('\x1b]8;;javascript:alert(1)\x1b\\text\x1b]8;;\x1b\\')).toEqual('text');

  // a sequence cut off by the line end is dropped, and style carries on to the next line
  const ansi = new AnsiLineRenderer();
  expect(renderAnsi('\x1b[31mred\x1b[38;5;', ansi)).toEqual('<span class="ansi-red-fg">red</span>');
  expect(renderAnsi('still red', ansi)).toEqual('<span class="ansi-red-fg">still red</span>');
});
