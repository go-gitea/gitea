import {
  basename, extname, isObject, uniq, stripTags, joinPaths, parseIssueHref, strSubMatch,
  prettyNumber, parseUrl,
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

test('joinPaths', () => {
  expect(joinPaths('', '')).toEqual('');
  expect(joinPaths('', 'b')).toEqual('b');
  expect(joinPaths('', '/b')).toEqual('/b');
  expect(joinPaths('', '/b/')).toEqual('/b/');
  expect(joinPaths('a', '')).toEqual('a');
  expect(joinPaths('/a', '')).toEqual('/a');
  expect(joinPaths('/a/', '')).toEqual('/a/');
  expect(joinPaths('a', 'b')).toEqual('a/b');
  expect(joinPaths('a', '/b')).toEqual('a/b');
  expect(joinPaths('/a', '/b')).toEqual('/a/b');
  expect(joinPaths('/a', '/b')).toEqual('/a/b');
  expect(joinPaths('/a/', '/b')).toEqual('/a/b');
  expect(joinPaths('/a', '/b/')).toEqual('/a/b/');
  expect(joinPaths('/a/', '/b/')).toEqual('/a/b/');

  expect(joinPaths('', '', '')).toEqual('');
  expect(joinPaths('', 'b', '')).toEqual('b');
  expect(joinPaths('', 'b', 'c')).toEqual('b/c');
  expect(joinPaths('', '', 'c')).toEqual('c');
  expect(joinPaths('', '/b', '/c')).toEqual('/b/c');
  expect(joinPaths('/a', '', '/c')).toEqual('/a/c');
  expect(joinPaths('/a', '/b', '')).toEqual('/a/b');

  expect(joinPaths('', '/')).toEqual('/');
  expect(joinPaths('a', '/')).toEqual('a/');
  expect(joinPaths('', '/', '/')).toEqual('/');
  expect(joinPaths('/', '/')).toEqual('/');
  expect(joinPaths('/', '')).toEqual('/');
  expect(joinPaths('/', 'b')).toEqual('/b');
  expect(joinPaths('/', 'b/')).toEqual('/b/');
  expect(joinPaths('/', '', '/')).toEqual('/');
  expect(joinPaths('/', 'b', '/')).toEqual('/b/');
  expect(joinPaths('/', 'b/', '/')).toEqual('/b/');
  expect(joinPaths('a', '/', '/')).toEqual('a/');
  expect(joinPaths('/', '/', 'c')).toEqual('/c');
  expect(joinPaths('/', '/', 'c/')).toEqual('/c/');
});

test('isObject', () => {
  expect(isObject({})).toBeTrue();
  expect(isObject([])).toBeFalse();
});

test('uniq', () => {
  expect(uniq([1, 1, 1, 2])).toEqual([1, 2]);
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

test('strSubMatch', () => {
  expect(strSubMatch('abc', '')).toEqual(['abc']);
  expect(strSubMatch('abc', 'a')).toEqual(['', 'a', 'bc']);
  expect(strSubMatch('abc', 'b')).toEqual(['a', 'b', 'c']);
  expect(strSubMatch('abc', 'c')).toEqual(['ab', 'c']);
  expect(strSubMatch('abc', 'ac')).toEqual(['', 'a', 'b', 'c']);
  expect(strSubMatch('abc', 'z')).toEqual(['abc']);
  expect(strSubMatch('abc', 'az')).toEqual(['abc']);

  expect(strSubMatch('aabbcc', 'abc')).toEqual(['', 'a', 'a', 'b', 'b', 'c', 'c']);
  expect(strSubMatch('the/directory', 'hedir')).toEqual(['t', 'he', '/', 'dir', 'ectory']);
});

test('prettyNumber', () => {
  expect(prettyNumber()).toEqual('');
  expect(prettyNumber(null)).toEqual('');
  expect(prettyNumber(undefined)).toEqual('');
  expect(prettyNumber('1200')).toEqual('');
  expect(prettyNumber(12345678, 'en-US')).toEqual('12,345,678');
  expect(prettyNumber(12345678, 'de-DE')).toEqual('12.345.678');
  expect(prettyNumber(12345678, 'be-BE')).toEqual('12 345 678');
  expect(prettyNumber(12345678, 'hi-IN')).toEqual('1,23,45,678');
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
