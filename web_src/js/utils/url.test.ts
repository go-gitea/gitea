import {pathEscapeSegments, isUrl} from './url.ts';

test('pathEscapeSegments', () => {
  expect(pathEscapeSegments('a/b/c')).toEqual('a/b/c');
  expect(pathEscapeSegments('a/b/ c')).toEqual('a/b/%20c');
});

test('isUrl', () => {
  expect(isUrl('https://example.com')).toEqual(true);
  expect(isUrl('https://example.com/')).toEqual(true);
  expect(isUrl('https://example.com/index.html')).toEqual(true);
  expect(isUrl('/index.html')).toEqual(false);
});
