import '../../../fomantic/build/fomantic.js';
import {initCodespaceSecretRepositoryPicker, initCodespaceTemplateEditor} from './CodespaceSettings.ts';
import {EditorView} from '@codemirror/view';
import * as modalModule from '../../modules/modal.ts';
import {initGiteaFomantic} from '../../modules/fomantic.ts';
import {userEvent} from 'vitest/browser';

vi.mock('../../modules/modal.ts', () => ({showModal: vi.fn()}));

beforeEach(() => vi.clearAllMocks());
beforeAll(() => {
  window.config.i18n.remove_label_str = 'Remove %s';
  initGiteaFomantic();
});
afterEach(() => {
  for (const dropdown of document.querySelectorAll('.ui.dropdown')) $(dropdown).data('module-dropdown').destroy();
  document.body.replaceChildren();
});

test('template editor submits edited JSONC and opens another template with fresh content', async () => {
  document.body.innerHTML = `
    <button class="codespace-template-edit" data-modal-form.url="/templates/1" data-modal-template-modal-title="Edit" data-submit-label="Save" data-modal-codespace-devcontainer-template-name="First"></button>
    <dialog open id="template-modal"><div class="header"></div><form>
      <input name="name"><div class="field"><div class="codespace-template-editor"><textarea name="content" data-code-editor-config='{"filename":"devcontainer.json"}'></textarea></div><div class="help">JSONC</div></div>
      <button class="cancel" type="button">Cancel</button><button class="ok"><span class="template-submit-label">Save</span></button>
    </form></dialog>`;
  const modal = document.querySelector<HTMLDialogElement>('#template-modal')!;
  const trigger = document.querySelector<HTMLButtonElement>('.codespace-template-edit')!;
  const first = '{\n// first\n"image":"debian:12",\n}';
  trigger.setAttribute('data-modal-codespace-devcontainer-template-content', first);
  let onHide: (() => void) | undefined;
  let shown = Promise.resolve();
  vi.mocked(modalModule.showModal).mockImplementation((_el, opts) => {
    onHide = () => opts!.onHidden!.call(modal);
    shown = Promise.resolve(opts!.onShow!.call(modal));
  });
  try {
    initCodespaceTemplateEditor(modal);
    trigger.click();
    const submit = modal.querySelector<HTMLButtonElement>('.ok')!;
    await shown;
    expect(submit.disabled).toBe(false);
    expect(submit.querySelector('.template-submit-label')!.textContent).toBe('Save');
    const view = EditorView.findFromDOM(modal.querySelector('.cm-editor')!)!;
    expect(view.state.doc.toString()).toBe(first);
    view.dispatch({changes: {from: 0, to: view.state.doc.length, insert: '{"image":"ubuntu:24.04"}'}});
    expect(new FormData(modal.querySelector('form')!).get('content')).toBe('{"image":"ubuntu:24.04"}');
    onHide!();
    trigger.setAttribute('data-modal-codespace-devcontainer-template-content', '{"image":"alpine:3"}');
    trigger.click();
    await shown;
    expect(submit.disabled).toBe(false);
    expect(EditorView.findFromDOM(modal.querySelector('.cm-editor')!)!.state.doc.toString()).toBe('{"image":"alpine:3"}');
  } finally {
    onHide!();
    vi.restoreAllMocks();
    document.body.replaceChildren();
  }
});

test('secret repository access restores selected repositories, switches scope and resets between edits', async () => {
  document.body.innerHTML = `
    <div class="codespace-secret-item">
      <span data-repository-id="1" data-repository-name="owner/first"></span>
      <span data-repository-id="2" data-repository-name="owner/second"></span>
      <button class="codespace-secret-access-button" data-modal-codespace-secret-access-all="false"></button>
    </div>
    <div class="codespace-secret-item">
      <span data-repository-id="3" data-repository-name="owner/third"></span>
      <button class="codespace-secret-access-button" data-modal-codespace-secret-access-all="false"></button>
    </div>
    <dialog open id="codespace-secret-access-modal">
      <form>
        <div class="codespace-secret-repository-picker" data-search-url="/repositories">
          <input class="codespace-secret-all-repositories" type="checkbox" name="all_repositories">
          <div class="codespace-secret-selected-scope">
            <select class="ui multiple search selection dropdown" name="repository_ids" multiple aria-label="Repository access"><option value="">Search repos</option></select>
          </div>
        </div>
      </form>
    </dialog>`;

  const picker = document.querySelector<HTMLElement>('.codespace-secret-repository-picker')!;
  initCodespaceSecretRepositoryPicker(picker);
  const triggers = document.querySelectorAll<HTMLButtonElement>('.codespace-secret-access-button');
  triggers[0].click();

  const form = document.querySelector('form')!;
  await expect.poll(() => new FormData(form).getAll('repository_ids')).toEqual(['1', '2']);
  expect(Array.from(picker.querySelectorAll('.ui.label'), (label) => label.textContent)).toEqual(['owner/first', 'owner/second']);

  const allRepositories = picker.querySelector<HTMLInputElement>('.codespace-secret-all-repositories')!;
  allRepositories.click();
  expect(picker.querySelector('.codespace-secret-selected-scope')!.classList.contains('tw-hidden')).toBe(true);
  expect(new FormData(form).getAll('repository_ids')).toEqual([]);
  expect(new FormData(form).has('all_repositories')).toBe(true);
  allRepositories.click();
  expect(new FormData(form).getAll('repository_ids')).toEqual(['1', '2']);

  triggers[1].click();
  await expect.poll(() => new FormData(form).getAll('repository_ids')).toEqual(['3']);
  expect(Array.from(picker.querySelectorAll('.ui.label'), (label) => label.textContent)).toEqual(['owner/third']);
});

test('secret creation searches repositories, submits multiple choices and preserves them on search failure', async () => {
  document.body.innerHTML = `
    <button class="codespace-secret-create-button"></button>
    <dialog open id="codespace-secret-create-modal"><form>
      <div class="codespace-secret-repository-picker" data-search-url="/repositories" data-no-results="No matching repositories" data-search-error="Search failed">
        <input class="codespace-secret-all-repositories" type="checkbox" name="all_repositories">
        <div class="codespace-secret-selected-scope">
          <select class="ui multiple search selection dropdown" name="repository_ids" multiple aria-label="Repository access"><option value="">Search repos</option></select>
        </div>
      </div>
    </form></dialog>`;
  const picker = document.querySelector<HTMLElement>('.codespace-secret-repository-picker')!;
  initCodespaceSecretRepositoryPicker(picker);
  const $dropdown = $(picker.querySelector('.ui.dropdown')!);
  $dropdown.dropdown('setting', {
    duration: 0,
    throttle: 0,
    apiSettings: {
      ...$dropdown.dropdown('setting', 'apiSettings'),
      mockResponseAsync(settings: {urlData: {query: string}}, callback: (response: unknown) => void) {
        const query = settings.urlData.query;
        if (query === 'error') callback(false);
        else if (query === 'missing') callback({data: []});
        else callback({data: query ? [{id: 2, full_name: 'owner/second'}] : [{id: 1, full_name: 'owner/first'}]});
      },
    },
  });
  const form = document.querySelector('form')!;
  const search = picker.querySelector<HTMLInputElement>('input.search')!;
  await userEvent.click(search);
  await expect.poll(() => picker.querySelector<HTMLElement>('.menu .item[data-value="1"]')).toBeTruthy();
  await userEvent.click(picker.querySelector<HTMLElement>('.menu .item[data-value="1"]')!);
  expect(new FormData(form).getAll('repository_ids')).toEqual(['1']);

  await userEvent.fill(search, 'second');
  await expect.poll(() => picker.querySelector<HTMLElement>('.menu .item[data-value="2"]')).toBeTruthy();
  await userEvent.click(picker.querySelector<HTMLElement>('.menu .item[data-value="2"]')!);
  expect(new FormData(form).getAll('repository_ids')).toEqual(['1', '2']);

  await userEvent.fill(search, 'missing');
  await expect.poll(() => picker.querySelector('.menu .message')?.textContent).toBe('No matching repositories');
  await userEvent.fill(search, 'error');
  await expect.poll(() => picker.querySelector('.menu .message')?.textContent).toBe('Search failed');
  expect(new FormData(form).getAll('repository_ids')).toEqual(['1', '2']);
  const keydown = vi.fn();
  document.addEventListener('keydown', keydown, {once: true});
  await userEvent.keyboard('{Escape}');
  expect(keydown.mock.calls[0][0].defaultPrevented).toBe(true);
  expect($dropdown.dropdown('is visible')).toBe(false);
  picker.querySelector('.ui.label[data-value="1"] .delete')!.dispatchEvent(new MouseEvent('click', {bubbles: true}));
  expect(new FormData(form).getAll('repository_ids')).toEqual(['2']);

  const modal = document.querySelector('dialog')!;
  modal.close();
  await expect.poll(() => search.value).toBe('');
  expect(picker.querySelector('.menu .item')).toBeNull();
  modal.show();
  document.querySelector<HTMLButtonElement>('.codespace-secret-create-button')!.click();
  await expect.poll(() => new FormData(form).getAll('repository_ids')).toEqual([]);
  expect(picker.querySelector('.ui.label')).toBeNull();
});
