import {beforeEach, describe, expect, test, vi} from 'vitest';
import {initAdminConfigs} from './config.ts';
import {POST} from '../../modules/fetch.ts';

vi.mock('../../modules/fetch.ts', () => ({
  POST: vi.fn(),
}));

vi.mock('../../modules/tippy.ts', () => ({
  showTemporaryTooltip: vi.fn(),
}));

function createPreviewDOM() {
  document.body.innerHTML = `
    <div class="page-content admin config">
      <form class="ui form" action="/-/admin/config/instance_notice" method="post">
        <textarea name="message">Initial message</textarea>
        <select name="level">
          <option value="info" selected>Info</option>
          <option value="success">Success</option>
          <option value="warning">Warning</option>
          <option value="danger">Danger</option>
        </select>
        <input type="checkbox" name="show_icon" value="true" checked>
      </form>
      <div id="instance-notice-preview" class="ui info message">
        <div id="instance-notice-preview-icon"></div>
        <div id="instance-notice-preview-content"></div>
      </div>
      <div id="instance-notice-preview-icons" class="tw-hidden">
        <span data-level="info"><svg data-icon="info"></svg></span>
        <span data-level="success"><svg data-icon="success"></svg></span>
        <span data-level="warning"><svg data-icon="warning"></svg></span>
        <span data-level="danger"><svg data-icon="danger"></svg></span>
      </div>
    </div>
  `;
}

describe('Admin Instance Notice Preview', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    createPreviewDOM();
  });

  test('renders markdown preview on input', async () => {
    vi.mocked(POST).mockResolvedValue({
      text: async () => '<p>Rendered message</p>',
    } as Response);

    initAdminConfigs();

    const messageInput = document.querySelector<HTMLTextAreaElement>('textarea[name="message"]')!;
    messageInput.value = 'Updated message';
    messageInput.dispatchEvent(new Event('input'));

    await Promise.resolve();
    await Promise.resolve();

    expect(POST).toHaveBeenCalledWith('/-/markup', expect.objectContaining({
      data: expect.any(FormData),
    }));

    const formData = vi.mocked(POST).mock.calls[0][1]?.data as FormData;
    expect(formData.get('mode')).toBe('comment');
    expect(formData.get('text')).toBe('Updated message');

    const previewContent = document.querySelector('#instance-notice-preview-content')!;
    expect(previewContent.innerHTML).toContain('Rendered message');
  });

  test('updates preview class and icon when level and icon toggle change', () => {
    initAdminConfigs();

    const levelSelect = document.querySelector<HTMLSelectElement>('select[name="level"]')!;
    const showIcon = document.querySelector<HTMLInputElement>('input[name="show_icon"]')!;
    const preview = document.querySelector<HTMLDivElement>('#instance-notice-preview')!;
    const previewIcon = document.querySelector<HTMLDivElement>('#instance-notice-preview-icon')!;

    levelSelect.value = 'danger';
    levelSelect.dispatchEvent(new Event('change'));
    expect(preview.classList.contains('negative')).toBe(true);
    expect(previewIcon.innerHTML).toContain('data-icon="danger"');

    showIcon.checked = false;
    showIcon.dispatchEvent(new Event('change'));
    expect(previewIcon.classList.contains('tw-hidden')).toBe(true);
  });

  test('queues a second render while first request is in flight and re-renders with latest text', async () => {
    let firstResolve: ((value: Response) => void) | undefined;
    const firstPending = new Promise<Response>((resolve) => {
      firstResolve = resolve;
    });

    vi.mocked(POST)
      .mockImplementationOnce(async () => await firstPending)
      .mockResolvedValueOnce({
        text: async () => '<p>Second render</p>',
      } as Response);

    initAdminConfigs();

    const messageInput = document.querySelector<HTMLTextAreaElement>('textarea[name="message"]')!;

    messageInput.value = 'First value';
    messageInput.dispatchEvent(new Event('input'));

    await Promise.resolve();

    messageInput.value = 'Latest value';
    messageInput.dispatchEvent(new Event('input'));

    firstResolve?.({
      text: async () => '<p>First render</p>',
    } as Response);

    for (let i = 0; i < 10 && vi.mocked(POST).mock.calls.length < 2; i++) {
      await new Promise((resolve) => {
        setTimeout(resolve, 0);
      });
    }

    expect(POST).toHaveBeenCalledTimes(2);
    const secondData = vi.mocked(POST).mock.calls[1][1]?.data as FormData;
    expect(secondData.get('text')).toBe('Latest value');

    const previewContent = document.querySelector('#instance-notice-preview-content')!;
    for (let i = 0; i < 10 && !previewContent.innerHTML.includes('Second render'); i++) {
      await new Promise((resolve) => {
        setTimeout(resolve, 0);
      });
    }
    expect(previewContent.innerHTML).toContain('Second render');
  });
});
