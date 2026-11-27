import {codeSuggestionInsertion, parseDiffLineHash} from './repo-diff-review.ts';

test('parses a diff line hash with a mixed-side range', () => {
  const fileHash = 'a'.repeat(40);
  expect(parseDiffLineHash(`#diff-${fileHash}L6-R7`)).toEqual({fileHash, start: 'L6', end: 'R7'});
});

test('inserts a suggestion with embedded backticks into an inline selection', () => {
  expect(codeSuggestionInsertion('Before selected after', 7, 15, 'first\n  second with ```')).toEqual({
    text: '\n````suggestion\nfirst\n  second with ```\n````\n',
    cursor: 46,
  });
});
