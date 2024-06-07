import {createElementFromHTML} from './dom.js';

test('createElementFromHTML', () => {
  expect(createElementFromHTML('<a>foo<span>bar</span></a>').textContent).toEqual('foobar');
});
