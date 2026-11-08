import {initEditPreviewTab} from './repo-editor.ts';
import {POST} from '../modules/fetch.ts';

vi.mock('../modules/fetch.ts', () => ({POST: vi.fn(), GET: vi.fn(), request: vi.fn()})); // fetch-action.ts (via repo-editor.ts) also imports GET/request from this module

const formHTML = `
  <form class="edit form">
    <div class="ui compact small menu repo-editor-menu" data-repo-link="/owner/repo" data-ref-sub-url="branch/main" data-branch-name="main">
      <a data-tab="preview"></a>
      <a data-tab="diff"></a>
    </div>
    <input id="tree_path" value="README.md">
    <div class="tab" data-tab="write"><textarea>old text</textarea></div>
    <div class="tab" data-tab="preview"></div>
    <div class="tab" data-tab="diff"></div>
  </form>
`;

function fakeResponse(text: string) {
  // text() must resolve in a microtask: a real Response body needs extra task turns in the browser runner, which makes the stale assertions racy
  return {ok: true, text: () => Promise.resolve(text)} as Response;
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((r) => {resolve = r});
  return {promise, resolve};
}

async function flush() {
  await new Promise((r) => setTimeout(r, 0));
}

function setup() {
  document.body.innerHTML = formHTML;
  const elForm = document.querySelector<HTMLFormElement>('.edit.form')!;
  initEditPreviewTab(elForm);
  return {
    elTextarea: elForm.querySelector<HTMLTextAreaElement>('.tab[data-tab="write"] textarea')!,
    elPreviewTab: elForm.querySelector<HTMLAnchorElement>('a[data-tab="preview"]')!,
    elPreviewPanel: elForm.querySelector('.tab[data-tab="preview"]')!,
    elDiffTab: elForm.querySelector<HTMLAnchorElement>('a[data-tab="diff"]')!,
    elDiffPanel: elForm.querySelector('.tab[data-tab="diff"]')!,
  };
}

describe('initEditPreviewTab', () => {
  test('preview renders a single click', async () => {
    const {elPreviewTab, elPreviewPanel} = setup();
    vi.mocked(POST).mockResolvedValueOnce(fakeResponse('rendered text'));
    elPreviewTab.click();
    await vi.waitFor(() => expect(elPreviewPanel.textContent).toContain('rendered text'));
  });

  test('preview discards a stale response arriving after a newer one', async () => {
    const {elTextarea, elPreviewTab, elPreviewPanel} = setup();
    const first = deferred<Response>();
    const second = deferred<Response>();
    vi.mocked(POST).mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    elTextarea.value = 'old text';
    elPreviewTab.click();
    elTextarea.value = 'new text';
    elPreviewTab.click();
    await flush();
    second.resolve(fakeResponse('rendered new text'));
    await flush();
    first.resolve(fakeResponse('rendered old text'));
    await flush();
    expect(elPreviewPanel.textContent).toContain('rendered new text');
    expect(elPreviewPanel.textContent).not.toContain('rendered old text');
  });

  test('diff discards a stale response arriving after a newer one', async () => {
    const {elTextarea, elDiffTab, elDiffPanel} = setup();
    const first = deferred<Response>();
    const second = deferred<Response>();
    vi.mocked(POST).mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    elTextarea.value = 'old text';
    elDiffTab.click();
    elTextarea.value = 'new text';
    elDiffTab.click();
    await flush();
    second.resolve(fakeResponse('diff new text'));
    await flush();
    first.resolve(fakeResponse('diff old text'));
    await flush();
    expect(elDiffPanel.textContent).toContain('diff new text');
    expect(elDiffPanel.textContent).not.toContain('diff old text');
  });
});
