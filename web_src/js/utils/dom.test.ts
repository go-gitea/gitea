import {createElementFromAttrs, createElementFromHTML, querySingleVisibleElem} from './dom.ts';

test('createElementFromHTML', () => {
  expect(createElementFromHTML('<a>foo<span>bar</span></a>').outerHTML).toEqual('<a>foo<span>bar</span></a>');
});

test('createElementFromAttrs', () => {
  const el = createElementFromAttrs('button', {
    id: 'the-id',
    class: 'cls-1 cls-2',
    disabled: true,
    checked: false,
    required: null,
    tabindex: 0,
    'data-foo': 'the-data',
  }, 'txt', createElementFromHTML('<span>inner</span>'));
  expect(el.outerHTML).toEqual('<button id="the-id" class="cls-1 cls-2" disabled="" tabindex="0" data-foo="the-data">txt<span>inner</span></button>');
});

test('querySingleVisibleElem', () => {
  let el = createElementFromHTML('<div><span>foo</span></div>');
  expect(querySingleVisibleElem(el, 'span').textContent).toEqual('foo');
  el = createElementFromHTML('<div><span style="display: none;">foo</span><span>bar</span></div>');
  expect(querySingleVisibleElem(el, 'span').textContent).toEqual('bar');
  el = createElementFromHTML('<div><span>foo</span><span>bar</span></div>');
  expect(() => querySingleVisibleElem(el, 'span')).toThrowError('Expected exactly one visible element');
});
