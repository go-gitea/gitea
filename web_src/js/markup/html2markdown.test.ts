import {convertHtmlToMarkdown} from './html2markdown.ts';
import {createElementFromHTML} from '../utils/dom.ts';

const h = createElementFromHTML;

test('convertHtmlToMarkdown', () => {
  expect(convertHtmlToMarkdown(h(`<h1>h</h1>`))).toBe('# h');
  expect(convertHtmlToMarkdown(h(`<strong>txt</strong>`))).toBe('**txt**');
  expect(convertHtmlToMarkdown(h(`<em>txt</em>`))).toBe('_txt_');
  expect(convertHtmlToMarkdown(h(`<del>txt</del>`))).toBe('~~txt~~');

  expect(convertHtmlToMarkdown(h(`<a href="link">txt</a>`))).toBe('[txt](link)');
  expect(convertHtmlToMarkdown(h(`<a href="https://link">https://link</a>`))).toBe('https://link');

  expect(convertHtmlToMarkdown(h(`<img src="link">`))).toBe('![image](link)');
  expect(convertHtmlToMarkdown(h(`<img src="link" alt="name">`))).toBe('![name](link)');
  expect(convertHtmlToMarkdown(h(`<img src="link" width="1" height="1">`))).toBe('<img alt="image" width="1" height="1" src="link">');

  expect(convertHtmlToMarkdown(h(`<p>txt</p>`))).toBe('txt\n');
  expect(convertHtmlToMarkdown(h(`<blockquote>a\nb</blockquote>`))).toBe('> a\n> b\n');

  expect(convertHtmlToMarkdown(h(`<ol><li>a<ul><li>b</li></ul></li></ol>`))).toBe('1. a\n    * b\n\n');
  expect(convertHtmlToMarkdown(h(`<ol><li><input checked>a</li></ol>`))).toBe('1. [x] a\n');
  expect(convertHtmlToMarkdown(h(`<ul><li><strong>a</strong></li><li><a href="link">b</a></li><li>c</li></ul>`))).toBe('* **a**\n* [b](link)\n* c\n');
});
