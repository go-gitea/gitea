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
  expect(renderAnsi('\x1b[48;5;88ma\x1b[38;208;48;5;159mb\x1b[m')).toEqual(`<span style="background-color: rgb(135, 0, 0);">a</span><span style="background-color: rgb(175, 255, 255);">b</span>`);

  // URLs in ANSI output become clickable links
  const link = (url: string) => `<a href="${url}" target="_blank">${url}</a>`;
  expect(renderAnsi('foo https://example.com bar')).toEqual(`foo ${link('https://example.com')} bar`);
  expect(renderAnsi('<https://example.com?a=b&c=d#h>')).toEqual(`&lt;${link('https://example.com?a=b&amp;c=d#h')}&gt;`);
  expect(renderAnsi('open https://example.com.')).toEqual(`open ${link('https://example.com')}.`);
  expect(renderAnsi('"https://example.com"')).toEqual(`"${link('https://example.com')}"`);
  expect(renderAnsi('\x1b[32mhttps://example.com\x1b[0m')).toEqual(`<span class="ansi-green-fg">${link('https://example.com')}</span>`);

  // attributes, faint nesting so its color mixes with the outer one, conceal emitting no foreground
  expect(renderAnsi('\x1b[1;5;8mx')).toEqual('<span class="ansi-bold ansi-blink ansi-conceal">x</span>');
  expect(renderAnsi('\x1b[6mx')).toEqual('<span class="ansi-blink">x</span>'); // 6 is the rapid blink
  expect(renderAnsi('\x1b[2;31mx')).toEqual('<span class="ansi-red-fg"><span class="ansi-faint">x</span></span>');
  expect(renderAnsi('\x1b[31;7mx')).toEqual('<span class="ansi-inverse-fg ansi-red-bg">x</span>');
  expect(renderAnsi('\x1b[4:3;9;53mx')).toEqual('<span class="ansi-underline ansi-line-through ansi-overline ansi-wavy">x</span>');
  expect(renderAnsi('\x1b[4;58;5;9mx')).toEqual('<span class="ansi-underline" style="text-decoration-color: var(--color-ansi-bright-red);">x</span>');

  // a color as ":" sub-parameters, with and without a color space id, not consuming the codes after
  expect(renderAnsi('\x1b[38:2::255:0:0ma\x1b[48:2:0:0:255mb')).toEqual('<span style="color: rgb(255, 0, 0);">a</span><span style="color: rgb(255, 0, 0); background-color: rgb(0, 0, 255);">b</span>');
  expect(renderAnsi('\x1b[1;38:5:9;4mx')).toEqual('<span class="ansi-bold ansi-underline ansi-bright-red-fg">x</span>');
  // a private CSI carries no style, even ending in "m", and does not split the run around it
  expect(renderAnsi('\x1b[31mred\x1b[>4;2m!')).toEqual('<span class="ansi-red-fg">red!</span>');

  // OSC 8 hyperlinks
  expect(renderAnsi('\x1b]8;;https://example.com\x1b\\text\x1b]8;;\x1b\\')).toEqual(`<a href="https://example.com" target="_blank">text</a>`);
  // only an "http(s)" hyperlink renders as one, and nothing in a log can become markup or an attribute
  expect(renderAnsi('\x1b]8;;javascript:alert(1)\x1b\\text\x1b]8;;\x1b\\')).toEqual('text');
  expect(renderAnsi('\x1b]8;;data:text/html,<script>\x1b\\text\x1b]8;;\x1b\\')).toEqual('text');
  expect(renderAnsi('\x1b]8;;https://x" onmouseover="alert(1)\x07<img src=x>\x1b]8;;\x07')).toEqual(`<a href="https://x&quot; onmouseover=&quot;alert(1)" target="_blank">&lt;img src=x&gt;</a>`);
  // a style inside the label renders instead of leaking, and one left open ends with the line
  expect(renderAnsi('\x1b]8;id=1;https://example.com\x07\x1b[31mred\x1b[0m\x1b]8;;\x07')).toEqual(`<a href="https://example.com" target="_blank"><span class="ansi-red-fg">red</span></a>`);
  expect(renderAnsi('\x1b]8;;https://example.com\x07https://other.com')).toEqual(`<a href="https://example.com" target="_blank">https://other.com</a>`);

  // sequences with no visual representation are dropped whole, payload and all, an unterminated one
  // up to the next escape or the line end
  expect(renderAnsi('\x1bPfoo\x1b\\a\x1b_G1;2\x1b\\b\x1b^priv\x1b\\c\x1bXsos\x1b\\d')).toEqual('abcd');
  expect(renderAnsi('\x1b(Ba\x1b#8b\x1b7c\x1b8d\x1b]0;unterminated')).toEqual('abcd');
  expect(renderAnsi('\x1b]0;unterminated\x1b\x1b[31mred')).toEqual('<span class="ansi-red-fg">red</span>');
  // a string sequence also ends at the 8-bit ST, which a runner may emit instead of "\x1b\\"
  expect(renderAnsi('\x1b]0;title\x9cvisible')).toEqual('visible');
  expect(renderAnsi('\x1b]8;;https://example.com\x9ctext\x1b]8;;\x9c')).toEqual(`<a href="https://example.com" target="_blank">text</a>`);
  // an OSC 11 background color query, a CSI window title push and an OSC 2 title
  expect(renderAnsi('\x1b]11;?\x1b\\\x1b[22;2t\x1b]2;🟡 a title\x1b\\go: downloading')).toEqual('go: downloading');

  // control characters never reach the output, a backspace moves the cursor back one column
  expect(renderAnsi('a\x07b\x00c\x7fd\x9be')).toEqual('abcde');
  expect(renderAnsi('Reading... 10%\b\b\b100%')).toEqual('Reading... 100%');
  expect(renderAnsi('abc\b\bx')).toEqual('axc');
  expect(renderAnsi('\b🟡\bx')).toEqual('x');
  // a character a terminal never shows takes no column, and the cursor reaches back over a style
  // change, so what it lands on keeps the newer style
  expect(renderAnsi('ab\x01c\b\bZ')).toEqual('aZc');
  expect(renderAnsi('abc\x1b[31m\b\bx')).toEqual('a<span class="ansi-red-fg">x</span>c');

  // a sequence cut off by the line end is dropped, and style carries on to the next line
  const ansi = new AnsiLineRenderer();
  expect(renderAnsi('\x1b[31mred\x1b[38;5;', ansi)).toEqual('<span class="ansi-red-fg">red</span>');
  expect(renderAnsi('still red', ansi)).toEqual('<span class="ansi-red-fg">still red</span>');
});
