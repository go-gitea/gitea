import {formatCitations} from './citation.ts';

test('formatCitations', async () => {
  const {apa, bibtex} = await formatCitations('cff-version: 1.2.0\ndoi: 10.1/a_b\nreferences: [{volume: 5}]', 'en-US');
  expect(apa).toBe('(Vol. 5). (n.d.-a).\n(N.d.-b). https://doi.org/10.1/a_b\n');
  expect(bibtex).toContain('doi = {10.1/a_b}');
});
