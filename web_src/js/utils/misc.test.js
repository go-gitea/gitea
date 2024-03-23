import {createLink} from './misc.js';

test('createLink', () => {
  const internalLink = createLink({href: 'https://example.com', textContent: 'example'});
  expect(internalLink.tagName).toEqual('A');
  expect(internalLink.href).toEqual('https://example.com/');
  expect(internalLink.textContent).toEqual('example');

  const externalLink = createLink({href: 'https://example.com', textContent: 'example', external: true});
  expect(externalLink.tagName).toEqual('A');
  expect(externalLink.href).toEqual('https://example.com/');
  expect(externalLink.textContent).toEqual('example');
  expect(externalLink.target).toEqual('_blank');
});
