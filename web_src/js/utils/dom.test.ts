import {
  createElementFromAttrs,
  createElementFromHTML,
  queryElemChildren,
  querySingleVisibleElem,
  toggleElem,
} from './dom.ts';

test('createElementFromHTML', () => {
  expect(createElementFromHTML('<a>foo<span>bar</span></a>').outerHTML).toEqual('<a>foo<span>bar</span></a>');
  expect(createElementFromHTML('<tr data-x="1"><td>foo</td></tr>').outerHTML).toEqual('<tr data-x="1"><td>foo</td></tr>');
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

test('queryElemChildren', () => {
  const el = createElementFromHTML('<div><span class="a">a</span><span class="b">b</span></div>');
  const children = queryElemChildren(el, '.a');
  expect(children.length).toEqual(1);
});

test('toggleElem', () => {
  const el = createElementFromHTML('<p><div>a</div><div class="tw-hidden">b</div></p>');
  toggleElem(el.children);
  expect(el.outerHTML).toEqual('<p><div class="tw-hidden">a</div><div class="">b</div></p>');
  toggleElem(el.children, false);
  expect(el.outerHTML).toEqual('<p><div class="tw-hidden">a</div><div class="tw-hidden">b</div></p>');
  toggleElem(el.children, true);
  expect(el.outerHTML).toEqual('<p><div class="">a</div><div class="">b</div></p>');
});
