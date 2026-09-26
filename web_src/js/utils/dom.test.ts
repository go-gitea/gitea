import {
  createElementFromAttrs, createElementFromHTML,
  queryElemChildren, querySingleVisibleElem,
  protectMorphElements, recoverMorphElements,
  toggleElem, morphElementWithProtection,
} from './dom.ts';

test('createElementFromHTML', () => {
  expect(createElementFromHTML('<a>foo<span>bar</span></a>').outerHTML).toEqual('<a>foo<span>bar</span></a>');
  expect(createElementFromHTML('<tr data-x="1"><td>foo</td></tr>').outerHTML).toEqual('<tr data-x="1"><td>foo</td></tr>');
  expect(createElementFromHTML('<TR data-x="1"><td>foo</td></TR>').outerHTML).toEqual('<tr data-x="1"><td>foo</td></tr>');
  expect(createElementFromHTML('<trx></trx>').outerHTML).toEqual('<trx></trx>');
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
  const el = document.createElement('div');
  document.body.append(el); // layout, and thus visibility, is only computed in the document
  expect(querySingleVisibleElem(el, 'span')).toBeNull();
  el.innerHTML = '<span>foo</span>';
  expect(querySingleVisibleElem(el, 'span')!.textContent).toEqual('foo');
  el.innerHTML = '<span style="display: none;">foo</span><span>bar</span>';
  expect(querySingleVisibleElem(el, 'span')!.textContent).toEqual('bar');
  el.innerHTML = '<span class="some-class tw-hidden">foo</span><span>bar</span>';
  expect(querySingleVisibleElem(el, 'span')!.textContent).toEqual('bar');
  el.innerHTML = '<span>foo</span><span>bar</span>';
  expect(() => querySingleVisibleElem(el, 'span')).toThrow('Expected exactly one visible element');
});

test('queryElemChildren', () => {
  const el = createElementFromHTML('<div><span class="a">a</span><span class="b">b</span></div>');
  const children = queryElemChildren(el, '.a');
  expect(children.length).toEqual(1);
});

test('toggleElem', () => {
  const el = createElementFromHTML('<div><div>a</div><div class="tw-hidden">b</div></div>');
  toggleElem(el.children);
  expect(el.outerHTML).toEqual('<div><div class="tw-hidden">a</div><div class="">b</div></div>');
  toggleElem(el.children, false);
  expect(el.outerHTML).toEqual('<div><div class="tw-hidden">a</div><div class="tw-hidden">b</div></div>');
  toggleElem(el.children, true);
  expect(el.outerHTML).toEqual('<div><div class="">a</div><div class="">b</div></div>');
});

test('protectMorphElements', () => {
  const el = createElementFromHTML('<div><span data-morph-protect="">foo</span></div>');
  const protectedElems = protectMorphElements(el);

  const span = el.querySelector('span')!;
  const spanMorphProtectId = span.getAttribute('data-morph-protect');
  expect(spanMorphProtectId).toBeTruthy();
  expect(el.outerHTML).toEqual(`<div><span data-morph-protect="${spanMorphProtectId}">foo</span></div>`);
  span.textContent = 'bar';
  span.classList.add('new-class');
  expect(el.outerHTML).toEqual(`<div><span data-morph-protect="${spanMorphProtectId}" class="new-class">bar</span></div>`);

  recoverMorphElements(el, protectedElems);
  expect(el.outerHTML).toEqual('<div><span data-morph-protect="">foo</span></div>');
});

describe('morphElementWithProtection', () => {
  const newElHtml = '<div><div><span>new</span><div class="ui dropdown">changed</div></div></div>';
  it('morph normal', () => {
    const el = createElementFromHTML('<div><div><span>old</span><div class="ui dropdown active"></div></div></div>');
    const newEl = createElementFromHTML(newElHtml);
    morphElementWithProtection(el, newEl, {morphStyle: 'outerHTML'});
    // span is changed, but dropdown is skipped because it is active
    expect(el.outerHTML).toEqual('<div><div><span>new</span><div class="ui dropdown active"></div></div></div>');
  });
  it('morph whole', () => {
    const el = createElementFromHTML('<div><div data-morph-whole><span>old</span><div class="ui dropdown active"></div></div></div>');
    const newEl = createElementFromHTML(newElHtml);
    morphElementWithProtection(el, newEl, {morphStyle: 'outerHTML'});
    // span is not changed, because the parent div is marked as data-morph-whole and there is an active dropdown, so it is skipped
    expect(el.outerHTML).toEqual('<div><div data-morph-whole=""><span>old</span><div class="ui dropdown active"></div></div></div>');
  });
  it('morph protection', () => {
    const el = createElementFromHTML('<div><span></span><div class="ui dropdown"></div></div>');
    const newEl = createElementFromHTML('<div><span>new</span><div class="ui dropdown">changed</div></div>');
    const elSpanOld = el.querySelector('span');
    const elDropdownOld = el.querySelector('.ui.dropdown');
    const morphedEl = morphElementWithProtection(el, newEl, {morphStyle: 'outerHTML'});
    expect(el.outerHTML).toEqual('<div><span>new</span><div class="ui dropdown">changed</div></div>');
    const elSpanNew = morphedEl.querySelector('span');
    const elDropdownNew = morphedEl.querySelector('.ui.dropdown');
    expect(elSpanNew).toBe(elSpanOld); // span is morphed in place, so it is the same element
    expect(elDropdownNew).not.toBe(elDropdownOld); // dropdown is protected and fully replaced, so it is a new element
  });
});
