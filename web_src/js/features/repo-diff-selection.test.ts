import {parseDiffLineHash} from './repo-diff-selection.ts';

test('parses a diff line hash with a mixed-side range', () => {
  const fileHash = 'a'.repeat(40);
  expect(parseDiffLineHash(`#diff-${fileHash}L6-R7`)).toEqual({fileHash, start: 'L6', end: 'R7'});
});
