import {execPseudoSelectorCommands, handleFetchActionErrorFields, handleFetchActionSuccessJson, performFetchAction} from './fetch-action.ts';
import {createElementFromHTML} from '../utils/dom.ts';
import {captureNavigations, normalizeTestHtml} from '../utils/testhelper.ts';

test('execPseudoSelectorCommands', () => {
  window.document.body.innerHTML = `
<div id="d1">
    <ul id="u1">
        <li class="x"></li>
    </ul>
    <ul id="u2">
        <li class="x"></li>
    </ul>
</div>
<div id="d2">
    <ul id="u3">
        <li class="x"></li>
    </ul>
</div>`;

  let ret = execPseudoSelectorCommands(document.querySelector('#u1')!, '');
  expect(ret.targets).toEqual([document.querySelector('#u1')]);

  ret = execPseudoSelectorCommands(document.querySelector('#u1')!, '$this');
  expect(ret.targets).toEqual([document.querySelector('#u1')]);
  expect(ret.cmdInnerHTML).toBeFalsy();
  expect(ret.cmdMorph).toBeFalsy();

  ret = execPseudoSelectorCommands(document.querySelector('#u1')!, '$body $morph $innerHTML');
  expect(ret.targets).toEqual([document.body]);
  expect(ret.cmdInnerHTML).toBeTruthy();
  expect(ret.cmdMorph).toBeTruthy();

  ret = execPseudoSelectorCommands(document.querySelector('#u1')!, '$body .x');
  expect(ret.targets.length).toEqual(3);
  expect(ret.targets).toEqual(Array.from(document.querySelectorAll('.x')));

  ret = execPseudoSelectorCommands(document.querySelector('#u1 .x')!, '$closest(div) .x');
  expect(ret.targets.length).toEqual(2);
  expect(ret.targets).toEqual(Array.from(document.querySelectorAll('#d1 .x')));
});

test('handleFetchActionSuccessJson', () => {
  const navigations = captureNavigations();
  handleFetchActionSuccessJson(document.body, {redirect: '/'});
  handleFetchActionSuccessJson(document.body, {redirect: ''});
  handleFetchActionSuccessJson(document.body, {});
  expect(navigations.map((n) => n.type)).toEqual(['push', 'reload', 'reload']);
});

test('morph keeps client-managed attributes while updating content', async () => {
  document.body.innerHTML = `
    <div id="history">
      <div id="details" class="open" data-fetch-morph-keep="class">Pending</div>
      <div data-fetch-trigger="every 5s"></div>
    </div>`;
  const response = `<div id="history"><div id="details" class="closed" data-fetch-morph-keep="class">Delivered</div></div>`;
  const fetchMock = vi.spyOn(window, 'fetch').mockResolvedValueOnce(new Response(response));
  try {
    const poll = document.querySelector<HTMLElement>('[data-fetch-trigger]')!;
    await performFetchAction(poll, {method: 'GET', url: '/history', successSync: '$closest(#history) $morph'});
    expect(document.querySelector('#details')!.className).toBe('open');
    expect(document.querySelector('#details')!.textContent).toBe('Delivered');
    expect(poll.isConnected).toBe(false);
  } finally {
    fetchMock.mockRestore();
  }
});

test('handleFetchActionErrorFields', () => {
  const elForm = createElementFromHTML<HTMLElement>(`<form>
<div class="error field"></div>
<div class="field"><input name="Foo_Bar[]"></div>
</form>`);
  handleFetchActionErrorFields(elForm, ['foo-BAR', 'other']);
  expect(normalizeTestHtml(elForm.outerHTML)).toEqual(normalizeTestHtml(`<form>
<div class="field"></div>
<div class="field error"><input name="Foo_Bar[]"></div>
</form>`));
});
