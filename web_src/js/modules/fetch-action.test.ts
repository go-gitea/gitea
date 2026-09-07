import {execPseudoSelectorCommands, handleFetchActionErrorFields, handleFetchActionSuccessJson} from './fetch-action.ts';
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

test('handleFetchActionSuccessJson', async () => {
  const navigations = captureNavigations();
  await handleFetchActionSuccessJson(document.body, {redirect: '/'});
  await handleFetchActionSuccessJson(document.body, {redirect: ''});
  await handleFetchActionSuccessJson(document.body, {});
  expect(navigations.map((n) => n.type)).toEqual(['push', 'reload', 'reload']);
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
