import {findUrlAtPosition, trimUrlPunctuation, urlRawRegex} from './utils.ts';

function matchUrls(text: string): string[] {
  return Array.from(text.matchAll(urlRawRegex), (m) => trimUrlPunctuation(m[0]));
}

test('matchUrls', () => {
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

test('findUrlAtPosition', () => {
  const doc = 'visit https://example.com for info';
  expect(findUrlAtPosition(doc, 0)).toBeNull();
  expect(findUrlAtPosition(doc, 6)).toEqual('https://example.com');
  expect(findUrlAtPosition(doc, 15)).toEqual('https://example.com');
  expect(findUrlAtPosition(doc, 25)).toEqual('https://example.com');
  expect(findUrlAtPosition(doc, 26)).toBeNull();
});
