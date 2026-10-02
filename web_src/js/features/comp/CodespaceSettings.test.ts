import {initCodespaceManagerSecretModal, initCodespaceSecretRepositoryPicker, initCodespaceTemplateEditor} from './CodespaceSettings.ts';
import {EditorView} from '@codemirror/view';
import {svg} from '../../svg.ts';
import * as modalModule from '../../modules/fomantic/modal.ts';
import * as fetchAction from '../../modules/fetch-action.ts';

vi.mock('../../modules/fomantic/modal.ts', () => ({showFomanticModal: vi.fn()}));
vi.mock('../../modules/fetch-action.ts', () => ({performFetchActionRequest: vi.fn()}));

beforeEach(() => vi.clearAllMocks());

test('template editor submits edited JSONC and opens another template with fresh content', async () => {
  document.body.innerHTML = `
    <button class="codespace-template-edit" data-modal-form.url="/templates/1" data-modal-template-modal-title="Edit" data-submit-label="Save" data-modal-codespace-devcontainer-template-name="First"></button>
    <div id="template-modal"><div class="header"></div><form>
      <input name="name"><div class="field"><div class="codespace-template-editor"><textarea name="content" data-code-editor-config='{"filename":"devcontainer.json"}'></textarea></div><div class="help">JSONC</div></div>
      <button class="cancel" type="button">Cancel</button><button class="ok">${svg('octicon-check')}<span class="template-submit-label">Save</span></button>
    </form></div>`;
  const modal = document.querySelector<HTMLElement>('#template-modal')!;
  const trigger = document.querySelector<HTMLButtonElement>('.codespace-template-edit')!;
  const first = '{\n// first\n"image":"debian:12",\n}';
  trigger.setAttribute('data-modal-codespace-devcontainer-template-content', first);
  let onHide: (() => void) | undefined;
  let shown = Promise.resolve();
  vi.mocked(modalModule.showFomanticModal).mockImplementation((_el, opts) => {
    onHide = () => opts!.onHide!.call(modal);
    shown = Promise.resolve(opts!.onShow!.call(modal));
  });
  try {
    initCodespaceTemplateEditor(modal);
    trigger.click();
    const submit = modal.querySelector<HTMLButtonElement>('.ok')!;
    await shown;
    expect(submit.disabled).toBe(false);
    expect(submit.querySelector('.svg.octicon-check')).not.toBeNull();
    expect(submit.querySelector('.template-submit-label')!.textContent).toBe('Save');
    expect(modal.querySelector('.code-editor-container')!.parentElement!.className).toBe('codespace-template-editor');
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

test('manager credentials are generated on demand, cleared on close, and confirmed before reset', async () => {
  document.body.innerHTML = `
    <button id="manager-secret-button" data-has-secret="false" data-reset-label="Reset">Generate</button>
    <p id="manager-secret-status" data-generated-label="Generated">Not generated</p>
    <div id="secret-modal">
      <p data-secret-confirm>Confirm reset</p>
      <div data-secret-result hidden>
        <input id="manager-secret-value"><button type="button" data-clipboard-target="#manager-secret-value">Copy</button>
      </div>
      <form action="/manager/1/secret"><button class="ui cancel button" type="button">Close</button><button class="ui primary button" data-secret-submit>Reset</button></form>
    </div>`;
  const modal = document.querySelector<HTMLElement>('#secret-modal')!;
  let onHide: (() => void) | undefined;
  const show = vi.mocked(modalModule.showFomanticModal).mockImplementation((_el, opts) => {
    onHide = () => opts!.onHide!.call(modal);
    opts!.onShow!.call(modal);
  });
  const request = vi.mocked(fetchAction.performFetchActionRequest).mockResolvedValue(Response.json({secret: 'first-secret'}));
  try {
    initCodespaceManagerSecretModal(modal);
    expect(show).not.toHaveBeenCalled();
    const trigger = document.querySelector<HTMLButtonElement>('#manager-secret-button')!;
    trigger.click();
    const value = modal.querySelector<HTMLInputElement>('input')!;
    await vi.waitFor(() => expect(value.value).toBe('first-secret'));
    expect(request.mock.calls[0][1].data).toBeInstanceOf(FormData);
    expect((request.mock.calls[0][1].data as FormData).has('confirm')).toBe(false);
    onHide!();
    expect(value.value).toBe('');
    trigger.click();
    expect(request).toHaveBeenCalledTimes(1);
    expect(modal.querySelector<HTMLElement>('[data-secret-confirm]')!.hidden).toBe(false);
    const submit = modal.querySelector<HTMLButtonElement>('[data-secret-submit]')!;
    expect(submit.classList.contains('tw-hidden')).toBe(false);
    request.mockResolvedValueOnce(null);
    const form = modal.querySelector('form')!;
    form.requestSubmit();
    await vi.waitFor(() => expect(trigger.disabled).toBe(false));
    expect(submit.classList.contains('tw-hidden')).toBe(false);
    expect((request.mock.calls[1][1].data as FormData).get('confirm')).toBe('reset-secret');
    request.mockResolvedValueOnce(Response.json({secret: 'second-secret'}));
    form.requestSubmit();
    await vi.waitFor(() => expect(value.value).toBe('second-secret'));
    expect(getComputedStyle(value).display).not.toBe('none');
    expect(value.getClientRects().length).toBeGreaterThan(0);
    expect(modal.querySelector<HTMLButtonElement>('[data-clipboard-target]')!.getClientRects().length).toBeGreaterThan(0);
    expect(modal.querySelector<HTMLElement>('[data-secret-confirm]')!.hidden).toBe(true);
    expect(submit.classList.contains('tw-hidden')).toBe(true);
    expect(form.querySelector<HTMLButtonElement>('.cancel')!.disabled).toBe(false);
    onHide!();
    expect(value.value).toBe('');
  } finally {
    vi.restoreAllMocks();
    document.body.replaceChildren();
  }
});

test('secret repository access loads the complete selected scope and switches to all repositories', () => {
  document.body.innerHTML = `
    <div class="codespace-secret-item">
      <span data-repository-id="1" data-repository-name="owner/first"></span>
      <span data-repository-id="2" data-repository-name="owner/second"></span>
      <button class="codespace-secret-access-button" data-modal-secret-access-all.checked="false"></button>
    </div>
    <div id="codespace-secret-access-modal">
      <form>
        <div class="codespace-secret-repository-picker" data-search-url="/repositories" data-remove-label="Remove">
          <input class="codespace-secret-all-repositories" type="checkbox">
          <div class="codespace-secret-selected-scope">
            <div class="codespace-secret-repository-search"><input class="prompt"></div>
            <div class="codespace-secret-selected-repositories"></div>
          </div>
        </div>
      </form>
    </div>`;

  const picker = document.querySelector<HTMLElement>('.codespace-secret-repository-picker')!;
  initCodespaceSecretRepositoryPicker(picker);
  document.querySelector<HTMLButtonElement>('.codespace-secret-access-button')!.click();

  const selected = Array.from(picker.querySelectorAll<HTMLInputElement>('input[name="repository_ids"]'), (input) => input.value);
  expect(selected).toEqual(['1', '2']);

  const allRepositories = picker.querySelector<HTMLInputElement>('.codespace-secret-all-repositories')!;
  allRepositories.checked = true;
  allRepositories.dispatchEvent(new Event('change'));
  expect(picker.querySelector('.codespace-secret-selected-scope')!.classList.contains('tw-hidden')).toBe(true);
});
