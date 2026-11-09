import {findLanguage, highlightLines, loadHighlighter} from './highlight.ts';

test('findLanguage', () => {
  expect(findLanguage('Go')).toEqual('go');
  expect(findLanguage('C++')).toEqual('cpp');
  expect(findLanguage('Shell')).toEqual('shellscript');
  expect(findLanguage('Fortran Free Form')).toEqual('fortran-free-form');
  expect(findLanguage('Jupyter Notebook')).toEqual('json');
  expect(findLanguage('Text')).toBeNull();
});

test('highlightLines', async () => {
  expect(await loadHighlighter('')).toBeNull();
  expect(await loadHighlighter('Unknown Language')).toBeNull();
  const highlighter = await loadHighlighter('Go');
  const lines = highlightLines(highlighter, ['func main() {', '', '\treturn // done', '}']);
  expect(lines.map((line) => line.map(([text]) => text).join(''))).toEqual(['func main() {', '', '\treturn // done', '}']);
  expect(lines[0]).toContainEqual(['func', 'tok-keyword']);
  expect(lines[0]).toContainEqual(['main', 'tok-function']);
  expect(lines[1]).toEqual([]);
  expect(lines[2]).toContainEqual(['return', 'tok-control']);
  expect(lines[2]).toContainEqual(['// done', 'tok-comment']);
  expect(lines[3]).toEqual([['}', 'tok-punctuation']]);
});
