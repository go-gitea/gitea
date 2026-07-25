import {AnsiLineRenderer} from './ansi.ts';

const renderAnsi = (line: string, ansi = new AnsiLineRenderer()) => {
  const el = document.createElement('div');
  ansi.renderInto(el, line);
  return el.innerHTML;
};

const link = (url: string) => `<a href="${url}" target="_blank">${url}</a>`;

test('renderAnsi', () => {
  const cases: Array<[string, string]> = [
    ['abc', 'abc'],
    ['abc\r\n', 'abc'],
    ['<script>', '&lt;script&gt;'],
    ['\rx\rabc', 'x\nabc'],
    ['a\x1b[Kb\x1b[2Jc', 'a\nb\nc'],
    ['a\x1b[?25lb', 'ab'],
    ['a\x1b]0;title\x07b', 'ab'],
    ['a\x1bMb', 'ab'],
    ['abc\x1b[3', 'abc'],
    ['open https://example.com.', `open ${link('https://example.com')}.`],
    ['\x1b]8;;https://example.com\x1b\\text\x1b]8;;\x1b\\', '<a href="https://example.com" target="_blank">text</a>'],
    ['\x1b]8;;ftp://example.com\x1b\\text\x1b]8;;\x1b\\', 'text'],
    ['\x1b[31mx', '<span class="ansi-red-fg">x</span>'],
    ['\x1b[41mx', '<span class="ansi-red-bg">x</span>'],
    ['\x1b[38;5;9mx', '<span class="ansi-bright-red-fg">x</span>'],
    ['\x1b[38;5;16mx', '<span style="color:#000000">x</span>'],
    ['\x1b[38;2;1;2;3mx', '<span style="color:#010203">x</span>'],
    ['\x1b[38;5;256mx', 'x'],
    ['\x1b[7mx', '<span class="ansi-inverse-fg ansi-inverse-bg">x</span>'],
    ['\x1b[31;7mx', '<span class="ansi-inverse-fg ansi-red-bg">x</span>'],
    ['\x1b[1mx', '<span class="ansi-bold">x</span>'],
    ['\x1b[5mx', '<span class="ansi-blink">x</span>'],
    ['\x1b[8mx', '<span class="ansi-conceal">x</span>'],
    ['\x1b[8;38;2;255;0;0mx', '<span class="ansi-conceal">x</span>'],
    ['\x1b[2;31mx', '<span class="ansi-red-fg"><span class="ansi-faint">x</span></span>'],
    ['\x1b[4;9;53mx', '<span class="ansi-underline ansi-line-through ansi-overline">x</span>'],
    ['\x1b[4:3mon\x1b[4:0moff', '<span class="ansi-underline ansi-wavy">on</span>off'],
    ['\x1b[4;58;2;135;0;255mx', '<span class="ansi-underline" style="text-decoration-color:#8700ff">x</span>'],
    ['\x1b[4;58;5;9mx', '<span class="ansi-underline" style="text-decoration-color:var(--color-ansi-bright-red)">x</span>'],
    ['\x1b[1mb\x1b[22mplain', '<span class="ansi-bold">b</span>plain'],
    ['\x1b[1;31mx\x1b[my', '<span class="ansi-bold ansi-red-fg">x</span>y'],
  ];
  for (const [input, expected] of cases) expect(renderAnsi(input), input).toEqual(expected);
});

test('renderAnsi carries style across lines', () => {
  const ansi = new AnsiLineRenderer();
  expect(renderAnsi('\x1b[31mred', ansi)).toEqual('<span class="ansi-red-fg">red</span>');
  expect(renderAnsi('still red', ansi)).toEqual('<span class="ansi-red-fg">still red</span>');
  expect(renderAnsi('\x1b[0m', ansi)).toEqual('');
  expect(renderAnsi('oops\x1b[3', ansi)).toEqual('oops');
  expect(renderAnsi('plain', ansi)).toEqual('plain');
});

test('renderAnsi does not allow html injection', () => {
  const scriptScheme = ['java', 'script:'].join(''); // a literal would trip "no-script-url"
  expect(renderAnsi('\x1b[31m"><img src=x onerror=alert(1)>')).toEqual('<span class="ansi-red-fg">"&gt;&lt;img src=x onerror=alert(1)&gt;</span>');
  expect(renderAnsi(`\x1b]8;;${scriptScheme}alert(1)\x1b\\label\x1b]8;;\x1b\\`)).toEqual('label');

  const el = document.createElement('div');
  new AnsiLineRenderer().renderInto(el, '\x1b]8;;https://x" onmouseover="alert(1)\x1b\\label\x1b]8;;\x1b\\');
  expect(el.querySelector('a')!.getAttribute('onmouseover')).toBeNull();
  expect(el.querySelector('a')!.getAttribute('href')).toEqual('https://x" onmouseover="alert(1)');
});
