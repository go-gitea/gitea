import {createElement, createElementFromHTML, elemGetAttributeNumber} from './dom.js';

test('createElementFromHTML', () => {
  expect(createElementFromHTML('<a>foo<span>bar</span></a>').outerHTML).toEqual('<a>foo<span>bar</span></a>');
});

test('createElement', () => {
  const el = createElement('button', {
    id: 'the-id',
    class: 'cls-1 cls-2',
    'data-foo': 'the-data',
    disabled: true,
    required: null,
  });
  expect(el.outerHTML).toEqual('<button id="the-id" class="cls-1 cls-2" data-foo="the-data" disabled=""></button>');
});

test('elemGetAttributeNumber', () => {
  expect(elemGetAttributeNumber(createElementFromHTML('<a>foo</a>'), `data-key`)).toEqual(null);
  expect(elemGetAttributeNumber(createElementFromHTML('<a>foo</a>'), `data-key`, 1)).toEqual(1);
  expect(elemGetAttributeNumber(createElementFromHTML('<a data-key="2">foo</a>'), `data-key`)).toEqual(2);
  expect(() => {
    elemGetAttributeNumber(createElementFromHTML('<a data-key="abc">foo</a>'), `data-key`);
  }).toThrowError('Attribute "data-key" is not a number');
});
