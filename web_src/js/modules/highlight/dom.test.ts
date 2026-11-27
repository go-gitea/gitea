import {highlightCodeElements} from './dom.ts';

test('highlightCodeElements', async () => {
  const container = document.createElement('div');
  container.innerHTML = '<code>func <mark>main</mark>() {\r\n</code><code>\treturn <span class="escaped-code-point">\u{200B}</span>// done\n</code><code>}</code>';
  const text = container.textContent;
  await highlightCodeElements(Array.from(container.querySelectorAll('code')), 'Go');
  expect(container.textContent).toEqual(text);
  expect(container.querySelector('.tok-keyword')!.textContent).toEqual('func');
  expect(container.querySelector('mark > .tok-function')!.textContent).toEqual('main');
  expect(container.querySelector('.escaped-code-point')!.textContent).toEqual('\u{200B}');
  expect(container.querySelector('.tok-comment')!.textContent).toEqual('// done');
  expect(container.querySelector('code:last-child > .tok-punctuation')!.textContent).toEqual('}');

  const plain = document.createElement('code');
  plain.textContent = 'a = 1';
  await highlightCodeElements([plain], 'Unknown Language');
  expect(plain.innerHTML).toEqual('a = 1');
});
