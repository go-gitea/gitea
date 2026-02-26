import {cleanUrl, findUrlAt, pathEscapeSegments, toOriginUrl, urlRawRegex} from './url.ts';

function matchUrls(text: string): string[] {
  return Array.from(text.matchAll(urlRawRegex), (m) => cleanUrl(m[0]));
}

test('urlRawRegex and cleanUrl', () => {
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

test('findUrlAt', () => {
  const doc = 'visit https://example.com for info';
  expect(findUrlAt(doc, 0)).toBeNull();
  expect(findUrlAt(doc, 6)).toEqual('https://example.com');
  expect(findUrlAt(doc, 15)).toEqual('https://example.com');
  expect(findUrlAt(doc, 25)).toEqual('https://example.com');
  expect(findUrlAt(doc, 26)).toBeNull();
});

test('pathEscapeSegments', () => {
  expect(pathEscapeSegments('a/b/c')).toEqual('a/b/c');
  expect(pathEscapeSegments('a/b/ c')).toEqual('a/b/%20c');
});

test('toOriginUrl', () => {
  const oldLocation = String(window.location);
  for (const origin of ['https://example.com', 'https://example.com:3000']) {
    window.location.assign(`${origin}/`);
    expect(toOriginUrl('/')).toEqual(`${origin}/`);
    expect(toOriginUrl('/org/repo.git')).toEqual(`${origin}/org/repo.git`);
    expect(toOriginUrl('https://another.com')).toEqual(`${origin}/`);
    expect(toOriginUrl('https://another.com/')).toEqual(`${origin}/`);
    expect(toOriginUrl('https://another.com/org/repo.git')).toEqual(`${origin}/org/repo.git`);
    expect(toOriginUrl('https://another.com:4000')).toEqual(`${origin}/`);
    expect(toOriginUrl('https://another.com:4000/')).toEqual(`${origin}/`);
    expect(toOriginUrl('https://another.com:4000/org/repo.git')).toEqual(`${origin}/org/repo.git`);
  }
  window.location.assign(oldLocation);
});
