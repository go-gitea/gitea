import {linkifyURLs, pathEscape, pathEscapeSegments, urlQueryEscape} from './url.ts';

describe('escape', () => {
  const queryNonAscii = " !\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~";
  test('urlQueryEscape', () => {
    const expected = '+%21%22%23%24%25%26%27%28%29%2A%2B%2C-.%2F%3A%3B%3C%3D%3E%3F%40%5B%5C%5D%5E_%60%7B%7C%7D~';
    expect(urlQueryEscape(queryNonAscii)).toEqual(expected);
  });

  test('pathEscape', () => {
    const expected = '%20%21%22%23$%25&%27%28%29%2A+%2C-.%2F:%3B%3C=%3E%3F@%5B%5C%5D%5E_%60%7B%7C%7D~';
    expect(pathEscape(queryNonAscii)).toEqual(expected);
  });

  test('pathEscapeSegments', () => {
    expect(pathEscapeSegments('a/b/c')).toEqual('a/b/c');
    expect(pathEscapeSegments('a/b/ c')).toEqual('a/b/%20c');
    expect(pathEscapeSegments('a/b+c')).toEqual('a/b+c');
  });
});

test('linkifyURLs', () => {
  const link = (url: string) => `<a href="${url}" target="_blank">${url}</a>`;
  expect(linkifyURLs('https://example.com')).toEqual(link('https://example.com'));
  expect(linkifyURLs('https://dl.google.com/go/go1.23.6.linux-amd64.tar.gz')).toEqual(link('https://dl.google.com/go/go1.23.6.linux-amd64.tar.gz'));
  expect(linkifyURLs('https://example.com/path?query=1&amp;b=2#frag')).toEqual(link('https://example.com/path?query=1&amp;b=2#frag'));
  expect(linkifyURLs('visit https://example.com/repo for info')).toEqual(`visit ${link('https://example.com/repo')} for info`);
  expect(linkifyURLs('See https://example.com.')).toEqual(`See ${link('https://example.com')}.`);
  expect(linkifyURLs('https://example.com, and more')).toEqual(`${link('https://example.com')}, and more`);
  expect(linkifyURLs('<span class="ansi-green-fg">https://proxy.golang.org/cached-only</span>')).toEqual(`<span class="ansi-green-fg">${link('https://proxy.golang.org/cached-only')}</span>`);
  expect(linkifyURLs('<span style="color:rgb(0,255,0)">https://registry.npmjs.org/@types/node</span>')).toEqual(`<span style="color:rgb(0,255,0)">${link('https://registry.npmjs.org/@types/node')}</span>`);
  expect(linkifyURLs('https://a.com and https://b.org')).toEqual(`${link('https://a.com')} and ${link('https://b.org')}`);
  expect(linkifyURLs('no urls here')).toEqual('no urls here');
  expect(linkifyURLs('http://example.com/path')).toEqual(link('http://example.com/path'));
  expect(linkifyURLs('http://localhost:3000/repo')).toEqual(link('http://localhost:3000/repo'));
  expect(linkifyURLs('https://')).toEqual('https://');
  expect(linkifyURLs('<a href="https://example.com">Click here</a>')).toEqual('<a href="https://example.com">Click here</a>');
  expect(linkifyURLs('<a\nhref="https://example.com">Click here</a>')).toEqual('<a\nhref="https://example.com">Click here</a>');
  expect(linkifyURLs('<a href="https://example.com">https://example.com</a>')).toEqual('<a href="https://example.com">https://example.com</a>');
  expect(linkifyURLs('https://evil.com/<script>alert(1)</script>')).toEqual(`${link('https://evil.com/')}<script>alert(1)</script>`);
  expect(linkifyURLs('https://evil.com/"onmouseover="alert(1)')).toEqual(`${link('https://evil.com/')}"onmouseover="alert(1)`);
  expect(linkifyURLs('javascript:alert(1)')).toEqual('javascript:alert(1)'); // eslint-disable-line no-script-url
  expect(linkifyURLs("https://evil.com/'onclick='alert(1)")).toEqual(`${link('https://evil.com/')}'onclick='alert(1)`);
  expect(linkifyURLs('data:text/html,<script>alert(1)</script>')).toEqual('data:text/html,<script>alert(1)</script>');
  expect(linkifyURLs('https://evil.com/\nonclick=alert(1)')).toEqual(`${link('https://evil.com/')}\nonclick=alert(1)`);
  expect(linkifyURLs('https://evil.com/&#34;onmouseover=alert(1)')).toEqual(`${link('https://evil.com/&#34;onmouseover=alert')}(1)`);
});
