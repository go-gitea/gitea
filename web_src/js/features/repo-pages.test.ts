import {DropzoneCustomEventUploadDone, initDropzone} from './dropzone.ts';
import {initRepoPagesSettings, initRepoPagesVersions} from './repo-pages.ts';

vi.mock('./dropzone.ts', () => ({
  DropzoneCustomEventUploadDone: 'dropzone-custom-upload-done',
  initDropzone: vi.fn(),
}));

beforeEach(() => {
  vi.mocked(initDropzone).mockReset();
});

describe('Repository Pages source form', () => {
  test('shows and enables only the selected source controls', () => {
    document.body.innerHTML = `
      <div id="pages-settings">
        <select id="pages-source-type">
          <option value="artifact" selected>Actions or upload</option>
          <option value="branch">Branch</option>
        </select>
    <div data-pages-source-panel="artifact"><input id="artifact"></div>
        <div data-pages-source-panel="branch"><input id="branch" disabled></div>
      </div>`;

    const root = document.querySelector<HTMLElement>('#pages-settings')!;
    const source = root.querySelector<HTMLSelectElement>('#pages-source-type')!;
    initRepoPagesSettings(root);

    expect(root.querySelector('#artifact')!.closest('[data-pages-source-panel]')!.classList.contains('tw-hidden')).toBe(false);
    expect(root.querySelector<HTMLInputElement>('#artifact')!.disabled).toBe(false);
    expect(root.querySelector<HTMLInputElement>('#branch')!.disabled).toBe(true);

    source.value = 'branch';
    source.dispatchEvent(new Event('change'));

    expect(root.querySelector('#artifact')!.closest('[data-pages-source-panel]')!.classList.contains('tw-hidden')).toBe(true);
    expect(root.querySelector<HTMLInputElement>('#artifact')!.disabled).toBe(true);
    expect(root.querySelector<HTMLInputElement>('#branch')!.disabled).toBe(false);
  });

  test('submits an archive after the staged upload completes', async () => {
    const listeners = new Map<string, () => void>();
    const dropzoneInstance = {
      on: vi.fn((event: string, listener: () => void) => {
        listeners.set(event, listener);
        return dropzoneInstance;
      }),
    };
    vi.mocked(initDropzone).mockResolvedValue(dropzoneInstance as never);

    document.body.innerHTML = `
    <div id="pages-versions">
    <form data-pages-upload>
          <div class="dropzone"></div>
        </form>
      </div>`;

    const root = document.querySelector<HTMLElement>('#pages-versions')!;
    const form = root.querySelector<HTMLFormElement>('[data-pages-upload]')!;
    const requestSubmit = vi.spyOn(form, 'requestSubmit').mockImplementation(() => {});
    await initRepoPagesVersions(root);

    listeners.get('addedfile')!();
    expect(root.querySelector('.dropzone')!.classList.contains('loading')).toBe(true);
    listeners.get('error')!();
    expect(root.querySelector('.dropzone')!.classList.contains('loading')).toBe(false);
    listeners.get(DropzoneCustomEventUploadDone)!();
    expect(requestSubmit).toHaveBeenCalledOnce();
  });
});
