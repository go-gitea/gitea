import {html, htmlEscape, htmlRaw} from './html.ts';

test('html', async () => {
  expect(html`<a>${'<>&\'"'}</a>`).toBe(`<a>&lt;&gt;&amp;&#39;&quot;</a>`);
  expect(html`<a>${htmlRaw('<img>')}</a>`).toBe(`<a><img></a>`);
  expect(html`<a>${htmlRaw`<img ${'&'}>`}</a>`).toBe(`<a><img &amp;></a>`);
  expect(htmlEscape(`<a></a>`)).toBe(`&lt;a&gt;&lt;/a&gt;`);
});
