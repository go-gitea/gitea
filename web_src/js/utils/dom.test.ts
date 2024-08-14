import {createElementFromAttrs, createElementFromHTML} from './dom.ts';

test('createElementFromHTML', () => {
  expect(createElementFromHTML('<a>foo<span>bar</span></a>').outerHTML).toEqual('<a>foo<span>bar</span></a>');
});

test('createElementFromAttrs', () => {
  const el = createElementFromAttrs('button', {
    id: 'the-id',
    class: 'cls-1 cls-2',
    'data-foo': 'the-data',
    disabled: true,
    checked: false,
    required: null,
    tabindex: 0,
  });
  expect(el.outerHTML).toEqual('<button id="the-id" class="cls-1 cls-2" data-foo="the-data" disabled="" tabindex="0"></button>');
});
