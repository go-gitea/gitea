import {
  basename, extname, isObject, uniq, stripTags,
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
  expect(isObject({})).toBeTrue();
  expect(isObject([])).toBeFalse();
});

test('uniq', () => {
  expect(uniq([1, 1, 1, 2])).toEqual([1, 2]);
});

test('stripTags', () => {
  expect(stripTags('<a>test</a>')).toEqual('test');
});
