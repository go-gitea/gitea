import {showTemporaryTooltip} from '../../modules/tippy.ts';
import {POST} from '../../modules/fetch.ts';
import {html, htmlRaw} from '../../utils/html.ts';

const {appSubUrl} = window.config;

function initInstanceNoticePreview(elAdminConfig: HTMLDivElement): void {
  const form = elAdminConfig.querySelector<HTMLFormElement>('form[action$="/-/admin/config/instance_notice"]');
  if (!form) return;

  const inputMessage = form.querySelector<HTMLTextAreaElement>('textarea[name="message"]');
  const selectLevel = form.querySelector<HTMLSelectElement>('select[name="level"]');
  const inputShowIcon = form.querySelector<HTMLInputElement>('input[name="show_icon"]');
  const preview = elAdminConfig.querySelector<HTMLDivElement>('#instance-notice-preview');
  const previewIcon = elAdminConfig.querySelector<HTMLDivElement>('#instance-notice-preview-icon');
  const previewContent = elAdminConfig.querySelector<HTMLDivElement>('#instance-notice-preview-content');
  const iconContainer = elAdminConfig.querySelector<HTMLDivElement>('#instance-notice-preview-icons');
  if (!inputMessage || !selectLevel || !inputShowIcon || !preview || !previewIcon || !previewContent || !iconContainer) return;

  const iconHTMLByLevel = new Map<string, string>();
  for (const el of iconContainer.querySelectorAll<HTMLElement>('[data-level]')) {
    iconHTMLByLevel.set(el.getAttribute('data-level')!, el.innerHTML);
  }

  const classByLevel: Record<string, string> = {
    info: 'info',
    success: 'positive',
    warning: 'warning',
    danger: 'negative',
  };

  const updateStyle = () => {
    preview.classList.remove('info', 'positive', 'warning', 'negative');
    preview.classList.add(classByLevel[selectLevel.value] || 'info');
    previewIcon.innerHTML = iconHTMLByLevel.get(selectLevel.value) || iconHTMLByLevel.get('info') || '';
    previewIcon.classList.toggle('tw-hidden', !inputShowIcon.checked);
  };

  let renderRequesting = false;
  let pendingRender = false;
  const renderPreviewMarkdown = async () => {
    if (renderRequesting) {
      pendingRender = true;
      return;
    }
    renderRequesting = true;
    try {
      while (true) {
        pendingRender = false;
        const formData = new FormData();
        formData.append('mode', 'comment');
        formData.append('text', inputMessage.value);
        try {
          const response = await POST(`${appSubUrl}/-/markup`, {data: formData});
          const rendered = await response.text();
          previewContent.innerHTML = html`${htmlRaw(rendered)}`;
        } catch (error) {
          console.error('Error rendering instance notice preview:', error);
        }
        if (!pendingRender) break;
      }
    } finally {
      renderRequesting = false;
    }
  };

  inputMessage.addEventListener('input', () => {
    renderPreviewMarkdown();
  });
  selectLevel.addEventListener('change', updateStyle);
  inputShowIcon.addEventListener('change', updateStyle);

  updateStyle();
}

export function initAdminConfigs(): void {
  const elAdminConfig = document.querySelector<HTMLDivElement>('.page-content.admin.config');
  if (!elAdminConfig) return;

  for (const el of elAdminConfig.querySelectorAll<HTMLInputElement>('input[type="checkbox"][data-config-dyn-key]')) {
    el.addEventListener('change', async () => {
      try {
        const resp = await POST(`${appSubUrl}/-/admin/config`, {
          data: new URLSearchParams({key: el.getAttribute('data-config-dyn-key')!, value: String(el.checked)}),
        });
        const json: Record<string, any> = await resp.json();
        if (json.errorMessage) throw new Error(json.errorMessage);
      } catch (ex) {
        showTemporaryTooltip(el, ex.toString());
        el.checked = !el.checked;
      }
    });
  }

  initInstanceNoticePreview(elAdminConfig);
}
