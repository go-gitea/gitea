import {initCodespaceTemplateEditor} from './CodespaceSettings.ts';
import {EditorView} from '@codemirror/view';
import * as modalModule from '../../modules/modal.ts';

vi.mock('../../modules/modal.ts', () => ({showModal: vi.fn()}));

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
