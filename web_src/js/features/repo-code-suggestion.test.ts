import {codeSuggestionInsertion} from './repo-code-suggestion.ts';

test('inserts a suggestion with embedded backticks into an inline selection', () => {
  expect(codeSuggestionInsertion('Before selected after', 7, 15, 'first\n  second with ```')).toEqual({
    text: '\n````suggestion\nfirst\n  second with ```\n````\n',
    cursor: 46,
  });
});
