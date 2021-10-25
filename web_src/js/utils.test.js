import {
  basename, extname, isObject, uniq, stripTags, joinPaths, parseIssueHref,
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
