import {createExternalLink} from '/misc.js';

test('createExternalLink', () => {
  const link = createExternalLink({href: 'https://example.com', textContent: 'example'});
  expect(link.tagName).toEqual('A');
  expect(link.href).toEqual('https://example.com/');
  expect(link.textContent).toEqual('example');
});
