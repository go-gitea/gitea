import {AnsiLineRenderer} from './ansi.ts';

const renderAnsi = (line: string, ansi = new AnsiLineRenderer()) => {
  const el = document.createElement('div');
  ansi.renderInto(el, line);
  return el.innerHTML;
};

test('renderAnsi text handling', () => {
  expect(renderAnsi('abc')).toEqual('abc');
  expect(renderAnsi('abc\n')).toEqual('abc');
  expect(renderAnsi('abc\r\n')).toEqual('abc');
  expect(renderAnsi('<script>')).toEqual('&lt;script&gt;');

  // a carriage return keeps only what each update wrote, and erase-line counts as one
  expect(renderAnsi('\r')).toEqual('');
  expect(renderAnsi('\rx\rabc')).toEqual('x\nabc');
  expect(renderAnsi('\rabc\rx\r')).toEqual('abc\nx');
  expect(renderAnsi('a\x1b[Kb\x1b[2Jc')).toEqual('a\nb\nc');
  expect(renderAnsi('\x1b[1A\x1b[2K\rtest\r\x1b[1B\x1b[1A\x1b[2K')).toEqual('test');

  // sequences that render nothing are dropped rather than shown as text
  expect(renderAnsi('a\x1b[?25lb')).toEqual('ab');
  expect(renderAnsi('a\x1b[1;2Hb')).toEqual('ab');
  expect(renderAnsi('a\x1b]0;window title\x07b')).toEqual('ab');
  expect(renderAnsi('a\x1bMb')).toEqual('ab');
  // a sequence cut off by the line end can never be completed
  expect(renderAnsi('abc\x1b[3')).toEqual('abc');
  expect(renderAnsi('abc\x1b')).toEqual('abc');

  // bare urls become links
  const link = (url: string) => `<a href="${url}" target="_blank">${url}</a>`;
  expect(renderAnsi('foo https://example.com bar')).toEqual(`foo ${link('https://example.com')} bar`);
  expect(renderAnsi('<https://example.com?a=b&c=d#h>')).toEqual(`&lt;${link('https://example.com?a=b&amp;c=d#h')}&gt;`);
  expect(renderAnsi('open https://example.com.')).toEqual(`open ${link('https://example.com')}.`);
});

test('renderAnsi colors', () => {
  // the 16 named colors carry a class, so a theme can restyle them
  expect(renderAnsi('\x1b[31mred')).toEqual('<span class="ansi-red-fg">red</span>');
  expect(renderAnsi('\x1b[41mbg')).toEqual('<span class="ansi-red-bg">bg</span>');
  expect(renderAnsi('\x1b[91mbright')).toEqual('<span class="ansi-bright-red-fg">bright</span>');
  expect(renderAnsi('\x1b[101mbright bg')).toEqual('<span class="ansi-bright-red-bg">bright bg</span>');
  expect(renderAnsi('\x1b[30mblack\x1b[37mwhite')).toEqual('<span class="ansi-black-fg">black</span><span class="ansi-white-fg">white</span>');
  expect(renderAnsi('\x1b[38;5;1mx')).toEqual('<span class="ansi-red-fg">x</span>');
  expect(renderAnsi('\x1b[38;5;9mx')).toEqual('<span class="ansi-bright-red-fg">x</span>');

  // the 256-color cube, grayscale and truecolor are literal, a theme must not restyle them
  expect(renderAnsi('\x1b[38;5;16mx')).toEqual('<span style="color:#000000">x</span>');
  expect(renderAnsi('\x1b[38;5;231mx')).toEqual('<span style="color:#ffffff">x</span>');
  expect(renderAnsi('\x1b[38;5;232mx')).toEqual('<span style="color:#080808">x</span>');
  expect(renderAnsi('\x1b[38;5;255mx')).toEqual('<span style="color:#eeeeee">x</span>');
  expect(renderAnsi('\x1b[38;2;1;2;3mx')).toEqual('<span style="color:#010203">x</span>');
  expect(renderAnsi('\x1b[48;2;1;2;3mx')).toEqual('<span style="background-color:#010203">x</span>');
  expect(renderAnsi('\x1b[48;5;88ma\x1b[38;208;48;5;159mb\x1b[m')).toEqual('<span style="background-color:#870000">a</span><span style="background-color:#afffff">b</span>');

  // out of range values are ignored rather than clamped
  expect(renderAnsi('\x1b[38;5;256mx')).toEqual('x');
  expect(renderAnsi('\x1b[38;2;300;0;0mx')).toEqual('x');

  // inverse swaps foreground and background, falling back to the console defaults
  expect(renderAnsi('\x1b[7mx')).toEqual('<span class="ansi-inverse-fg ansi-inverse-bg">x</span>');
  expect(renderAnsi('\x1b[31;7mx')).toEqual('<span class="ansi-inverse-fg ansi-red-bg">x</span>');
  expect(renderAnsi('\x1b[31;42;7mx')).toEqual('<span class="ansi-green-fg ansi-red-bg">x</span>');
  expect(renderAnsi('\x1b[7mon\x1b[27moff')).toEqual('<span class="ansi-inverse-fg ansi-inverse-bg">on</span>off');
});

test('renderAnsi attributes', () => {
  expect(renderAnsi('\x1b[1mx')).toEqual('<span class="ansi-bold">x</span>');
  expect(renderAnsi('\x1b[3mx')).toEqual('<span class="ansi-italic">x</span>');
  expect(renderAnsi('\x1b[8mx')).toEqual('<span class="ansi-conceal">x</span>');
  expect(renderAnsi('\x1b[4mx')).toEqual('<span class="ansi-underline">x</span>');
  expect(renderAnsi('\x1b[9mx')).toEqual('<span class="ansi-line-through">x</span>');
  expect(renderAnsi('\x1b[53mx')).toEqual('<span class="ansi-overline">x</span>');
  expect(renderAnsi('\x1b[4;9;53mx')).toEqual('<span class="ansi-underline ansi-line-through ansi-overline">x</span>');

  // faint nests, so its translucent color mixes with whatever color the outer span applies
  expect(renderAnsi('\x1b[2mx')).toEqual('<span class="ansi-faint">x</span>');
  expect(renderAnsi('\x1b[2;31mx')).toEqual('<span class="ansi-red-fg"><span class="ansi-faint">x</span></span>');

  // colon sub-parameters select the underline style, "4:0" turns it off
  expect(renderAnsi('\x1b[4:2mx')).toEqual('<span class="ansi-underline ansi-double">x</span>');
  expect(renderAnsi('\x1b[4:3mx')).toEqual('<span class="ansi-underline ansi-wavy">x</span>');
  expect(renderAnsi('\x1b[4:5mx')).toEqual('<span class="ansi-underline ansi-dashed">x</span>');
  expect(renderAnsi('\x1b[4:3mon\x1b[4:0moff')).toEqual('<span class="ansi-underline ansi-wavy">on</span>off');

  // 58 colors the underline and 59 restores the default, their parameters are never read as codes
  expect(renderAnsi('\x1b[4;58;2;135;0;255mx')).toEqual('<span class="ansi-underline" style="text-decoration-color:#8700ff">x</span>');
  expect(renderAnsi('\x1b[4;58;5;9mx')).toEqual('<span class="ansi-underline" style="text-decoration-color:var(--color-ansi-bright-red)">x</span>');
  expect(renderAnsi('\x1b[4;58;2;1;2;3mon\x1b[59moff')).toEqual('<span class="ansi-underline" style="text-decoration-color:#010203">on</span><span class="ansi-underline">off</span>');

  // each attribute has a reset, and "\x1b[m" resets everything like "\x1b[0m"
  expect(renderAnsi('\x1b[1mb\x1b[22mplain')).toEqual('<span class="ansi-bold">b</span>plain');
  expect(renderAnsi('\x1b[31mr\x1b[39mplain')).toEqual('<span class="ansi-red-fg">r</span>plain');
  expect(renderAnsi('\x1b[9mon\x1b[29moff')).toEqual('<span class="ansi-line-through">on</span>off');
  expect(renderAnsi('\x1b[1;31mx\x1b[my')).toEqual('<span class="ansi-bold ansi-red-fg">x</span>y');
});

test('AnsiLineRenderer carries style across lines', () => {
  const ansi = new AnsiLineRenderer();
  // an unterminated color keeps applying to the following lines, including plain ones
  expect(renderAnsi('\x1b[31mred', ansi)).toEqual('<span class="ansi-red-fg">red</span>');
  expect(renderAnsi('still red', ansi)).toEqual('<span class="ansi-red-fg">still red</span>');
  expect(renderAnsi('\x1b[1mand bold', ansi)).toEqual('<span class="ansi-bold ansi-red-fg">and bold</span>');
  // until it is reset, and a truncated sequence never carries
  expect(renderAnsi('\x1b[0m', ansi)).toEqual('');
  expect(renderAnsi('oops\x1b[3', ansi)).toEqual('oops');
  expect(renderAnsi('plain', ansi)).toEqual('plain');
});

test('renderAnsi does not allow html injection', () => {
  const esc = '\x1b';
  const scriptScheme = ['java', 'script:'].join(''); // assembled, a literal would trip "no-script-url"
  const link = (url: string, label = 'label') => `${esc}]8;;${url}${esc}\\${label}${esc}]8;;${esc}\\`;
  const payloads = [
    '<script>alert(1)</script>',
    '<img src=x onerror=alert(1)>',
    `${esc}[31m<script>alert(1)</script>`,
    `${esc}[31m"><script>alert(1)</script>`,
    `${esc}[31m</span><script>alert(1)</script>`,
    `${esc}[<script>malicious`, // sgr parameters are parsed, never emitted
    // hyperlink targets that must not become a live href
    link(`${scriptScheme}alert(1)`),
    link(`${scriptScheme.toUpperCase()}alert(1)`),
    link('data:text/html,<script>alert(1)</script>'),
    link(` ${scriptScheme}alert(1)`),
    link('//evil.example.com'),
    // attempts to break out of the href attribute, and markup in the label
    link('https://x" onmouseover="alert(1)'),
    link('https://x"><script>alert(1)</script>'),
    link('https://example.com', '"><img src=x onerror=alert(1)>'),
    // bare urls, which the url post-process turns into links
    `see ${scriptScheme}alert(1) here`,
    'see https://x"><script>alert(1)</script> here',
  ];

  for (const payload of payloads) {
    const el = document.createElement('div');
    new AnsiLineRenderer().renderInto(el, payload);
    expect(el.querySelectorAll('script, iframe, img, svg, object, embed'), payload).toHaveLength(0);
    for (const node of el.querySelectorAll('*')) {
      expect([...node.attributes].filter((attr) => attr.name.startsWith('on')), payload).toEqual([]);
    }
    for (const anchor of el.querySelectorAll('a')) {
      expect(anchor.getAttribute('href'), payload).toMatch(/^https?:\/\//);
    }
    // a serialise and parse round trip is where mutation xss shows up
    const reparsed = document.createElement('div');
    reparsed.innerHTML = el.innerHTML;
    expect(reparsed.querySelectorAll('script, iframe, img, svg, object, embed'), payload).toHaveLength(0);
    expect(reparsed.textContent, payload).toEqual(el.textContent);
  }
});
