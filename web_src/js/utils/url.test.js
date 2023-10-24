import {pathEscapeSegments} from './url.js';

test('pathEscapeSegments', () => {
  expect(pathEscapeSegments('a/b/c')).toEqual('a/b/c');
  expect(pathEscapeSegments('a/b/ c')).toEqual('a/b/%20c');
});
