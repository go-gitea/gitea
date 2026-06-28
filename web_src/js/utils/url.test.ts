import {pathEscape, pathEscapeSegments, trimUrlPunctuation, urlQueryEscape, urlRawRegex} from './url.ts';

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

test('matchUrls', () => {
  const matchUrls = (text: string) => Array.from(text.matchAll(urlRawRegex), (m) => trimUrlPunctuation(m[0]));
  expect(matchUrls('visit https://example.com for info')).toEqual(['https://example.com']);
  expect(matchUrls('see https://example.com.')).toEqual(['https://example.com']);
  expect(matchUrls('see https://example.com, and')).toEqual(['https://example.com']);
  expect(matchUrls('see https://example.com; and')).toEqual(['https://example.com']);
  expect(matchUrls('(https://example.com)')).toEqual(['https://example.com']);
  expect(matchUrls('"https://example.com"')).toEqual(['https://example.com']);
  expect(matchUrls('https://example.com/path?q=1&b=2#hash')).toEqual(['https://example.com/path?q=1&b=2#hash']);
  expect(matchUrls('https://example.com/path?q=1&b=2#hash.')).toEqual(['https://example.com/path?q=1&b=2#hash']);
  expect(matchUrls('https://x.co')).toEqual(['https://x.co']);
  expect(matchUrls('https://example.com/path_(wiki)')).toEqual(['https://example.com/path_(wiki)']);
  expect(matchUrls('https://en.wikipedia.org/wiki/Rust_(programming_language)')).toEqual(['https://en.wikipedia.org/wiki/Rust_(programming_language)']);
  expect(matchUrls('(https://en.wikipedia.org/wiki/Rust_(programming_language))')).toEqual(['https://en.wikipedia.org/wiki/Rust_(programming_language)']);
  expect(matchUrls('http://example.com')).toEqual(['http://example.com']);
  expect(matchUrls('no url here')).toEqual([]);
  expect(matchUrls('https://a.com and https://b.com')).toEqual(['https://a.com', 'https://b.com']);
  expect(matchUrls('[![](https://img.shields.io/npm/v/pkg.svg?style=flat)](https://www.npmjs.org/package/pkg)')).toEqual(['https://img.shields.io/npm/v/pkg.svg?style=flat', 'https://www.npmjs.org/package/pkg']);
});

test('trimUrlPunctuation', () => {
  expect(trimUrlPunctuation('https://example.com.')).toEqual('https://example.com');
  expect(trimUrlPunctuation('https://example.com,')).toEqual('https://example.com');
  expect(trimUrlPunctuation('https://example.com;')).toEqual('https://example.com');
  expect(trimUrlPunctuation('https://example.com:')).toEqual('https://example.com');
  expect(trimUrlPunctuation("https://example.com'")).toEqual('https://example.com');
  expect(trimUrlPunctuation('https://example.com"')).toEqual('https://example.com');
  expect(trimUrlPunctuation('https://example.com.,;')).toEqual('https://example.com');
  expect(trimUrlPunctuation('https://example.com/path')).toEqual('https://example.com/path');
  expect(trimUrlPunctuation('https://example.com/path_(wiki)')).toEqual('https://example.com/path_(wiki)');
  expect(trimUrlPunctuation('https://example.com)')).toEqual('https://example.com');
  expect(trimUrlPunctuation('https://en.wikipedia.org/wiki/Rust_(lang))')).toEqual('https://en.wikipedia.org/wiki/Rust_(lang)');
});
