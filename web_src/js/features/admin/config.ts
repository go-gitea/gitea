import {showTemporaryTooltip} from '../../modules/tippy.ts';
import {POST} from '../../modules/fetch.ts';

const {appSubUrl} = window.config;

export function initAdminConfigs(): void {
  const elAdminConfig = document.querySelector<HTMLDivElement>('.page-content.admin.config');
  if (!elAdminConfig) return;

  for (const el of elAdminConfig.querySelectorAll<HTMLInputElement>('input[type="checkbox"][data-config-dyn-key]')) {
    el.addEventListener('change', async () => {
      try {
        const resp = await POST(`${appSubUrl}/-/admin/config`, {
          data: new URLSearchParams({key: el.getAttribute('data-config-dyn-key'), value: String(el.checked)}),
        });
        const json: Record<string, any> = await resp.json();
        if (json.errorMessage) throw new Error(json.errorMessage);
      } catch (ex) {
        showTemporaryTooltip(el, ex.toString());
        el.checked = !el.checked;
      }
    });
  }
}
