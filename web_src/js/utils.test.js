import {
  basename, extname, isObject, stripTags, parseIssueHref,
  parseUrl, translateMonth, translateDay, blobToDataURI,
  toAbsoluteUrl, encodeURLEncodedBase64, decodeURLEncodedBase64,
} from './utils.js';

test('basename', () => {
  expect(basename('/path/to/file.js')).toEqual('file.js');
  expect(basename('/path/to/file')).toEqual('file');
  expect(basename('file.js')).toEqual('file.js');
});

test('extname', () => {
  expect(extname('/path/to/file.js')).toEqual('.js');
  expect(extname('/path/')).toEqual('');
  expect(extname('/path')).toEqual('');
  expect(extname('file.js')).toEqual('.js');
});

test('isObject', () => {
  expect(isObject({})).toBeTruthy();
  expect(isObject([])).toBeFalsy();
});

test('stripTags', () => {
  expect(stripTags('<a>test</a>')).toEqual('test');
});

test('parseIssueHref', () => {
  expect(parseIssueHref('/owner/repo/issues/1')).toEqual({owner: 'owner', repo: 'repo', type: 'issues', index: '1'});
  expect(parseIssueHref('/owner/repo/pulls/1?query')).toEqual({owner: 'owner', repo: 'repo', type: 'pulls', index: '1'});
  expect(parseIssueHref('/owner/repo/issues/1#hash')).toEqual({owner: 'owner', repo: 'repo', type: 'issues', index: '1'});
  expect(parseIssueHref('/sub/owner/repo/issues/1')).toEqual({owner: 'owner', repo: 'repo', type: 'issues', index: '1'});
  expect(parseIssueHref('/sub/sub2/owner/repo/pulls/1')).toEqual({owner: 'owner', repo: 'repo', type: 'pulls', index: '1'});
  expect(parseIssueHref('/sub/sub2/owner/repo/issues/1?query')).toEqual({owner: 'owner', repo: 'repo', type: 'issues', index: '1'});
  expect(parseIssueHref('/sub/sub2/owner/repo/issues/1#hash')).toEqual({owner: 'owner', repo: 'repo', type: 'issues', index: '1'});
  expect(parseIssueHref('https://example.com/owner/repo/issues/1')).toEqual({owner: 'owner', repo: 'repo', type: 'issues', index: '1'});
  expect(parseIssueHref('https://example.com/owner/repo/pulls/1?query')).toEqual({owner: 'owner', repo: 'repo', type: 'pulls', index: '1'});
  expect(parseIssueHref('https://example.com/owner/repo/issues/1#hash')).toEqual({owner: 'owner', repo: 'repo', type: 'issues', index: '1'});
  expect(parseIssueHref('https://example.com/sub/owner/repo/issues/1')).toEqual({owner: 'owner', repo: 'repo', type: 'issues', index: '1'});
  expect(parseIssueHref('https://example.com/sub/sub2/owner/repo/pulls/1')).toEqual({owner: 'owner', repo: 'repo', type: 'pulls', index: '1'});
  expect(parseIssueHref('https://example.com/sub/sub2/owner/repo/issues/1?query')).toEqual({owner: 'owner', repo: 'repo', type: 'issues', index: '1'});
  expect(parseIssueHref('https://example.com/sub/sub2/owner/repo/issues/1#hash')).toEqual({owner: 'owner', repo: 'repo', type: 'issues', index: '1'});
  expect(parseIssueHref('')).toEqual({owner: undefined, repo: undefined, type: undefined, index: undefined});
});

test('parseUrl', () => {
  expect(parseUrl('').pathname).toEqual('/');
  expect(parseUrl('/path').pathname).toEqual('/path');
  expect(parseUrl('/path?search').pathname).toEqual('/path');
  expect(parseUrl('/path?search').search).toEqual('?search');
  expect(parseUrl('/path?search#hash').hash).toEqual('#hash');
  expect(parseUrl('https://localhost/path').pathname).toEqual('/path');
  expect(parseUrl('https://localhost/path?search').pathname).toEqual('/path');
  expect(parseUrl('https://localhost/path?search').search).toEqual('?search');
  expect(parseUrl('https://localhost/path?search#hash').hash).toEqual('#hash');
});

test('translateMonth', () => {
  const originalLang = document.documentElement.lang;
  document.documentElement.lang = 'en-US';
  expect(translateMonth(0)).toEqual('Jan');
  expect(translateMonth(4)).toEqual('May');
  document.documentElement.lang = 'es-ES';
  expect(translateMonth(5)).toEqual('jun');
  expect(translateMonth(6)).toEqual('jul');
  document.documentElement.lang = originalLang;
});

test('translateDay', () => {
  const originalLang = document.documentElement.lang;
  document.documentElement.lang = 'fr-FR';
  expect(translateDay(1)).toEqual('lun.');
  expect(translateDay(5)).toEqual('ven.');
  document.documentElement.lang = 'pl-PL';
  expect(translateDay(1)).toEqual('pon.');
  expect(translateDay(5)).toEqual('pt.');
  document.documentElement.lang = originalLang;
});

test('blobToDataURI', async () => {
  const blob = new Blob([JSON.stringify({test: true})], {type: 'application/json'});
  expect(await blobToDataURI(blob)).toEqual('data:application/json;base64,eyJ0ZXN0Ijp0cnVlfQ==');
});

test('toAbsoluteUrl', () => {
  expect(toAbsoluteUrl('//host/dir')).toEqual('http://host/dir');
  expect(toAbsoluteUrl('https://host/dir')).toEqual('https://host/dir');

  expect(toAbsoluteUrl('')).toEqual('http://localhost:3000');
  expect(toAbsoluteUrl('/user/repo')).toEqual('http://localhost:3000/user/repo');

  expect(() => toAbsoluteUrl('path')).toThrowError('unsupported');
});

test('encodeURLEncodedBase64, decodeURLEncodedBase64', () => {
  // TextEncoder is Node.js API while Uint8Array is jsdom API and their outputs are not
  // structurally comparable, so we convert to array to compare. The conversion can be
  // removed once https://github.com/jsdom/jsdom/issues/2524 is resolved.
  const encoder = new TextEncoder();
  const uint8array = encoder.encode.bind(encoder);

  expect(encodeURLEncodedBase64(uint8array('AA?'))).toEqual('QUE_'); // standard base64: "QUE/"
  expect(encodeURLEncodedBase64(uint8array('AA~'))).toEqual('QUF-'); // standard base64: "QUF+"

  expect(Array.from(decodeURLEncodedBase64('QUE/'))).toEqual(Array.from(uint8array('AA?')));
  expect(Array.from(decodeURLEncodedBase64('QUF+'))).toEqual(Array.from(uint8array('AA~')));
  expect(Array.from(decodeURLEncodedBase64('QUE_'))).toEqual(Array.from(uint8array('AA?')));
  expect(Array.from(decodeURLEncodedBase64('QUF-'))).toEqual(Array.from(uint8array('AA~')));

  expect(encodeURLEncodedBase64(uint8array('a'))).toEqual('YQ'); // standard base64: "YQ=="
  expect(Array.from(decodeURLEncodedBase64('YQ'))).toEqual(Array.from(uint8array('a')));
  expect(Array.from(decodeURLEncodedBase64('YQ=='))).toEqual(Array.from(uint8array('a')));
});
