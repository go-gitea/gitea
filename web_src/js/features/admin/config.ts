import {showTemporaryTooltip} from '../../modules/tippy.ts';
import {POST} from '../../modules/fetch.ts';
import {fomanticQuery} from '../../modules/fomantic/base.ts';

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

  // Handle theme config dropdowns
  for (const el of elAdminConfig.querySelectorAll('.js-theme-config-dropdown')) {
    fomanticQuery(el).dropdown({
      async onChange(value: string, _text: string, _$item: any) {
        if (!value) return;

        const configKey = this.getAttribute('data-config-key');
        if (!configKey) return;

        try {
          const resp = await POST(`${appSubUrl}/-/admin/config`, {
            data: new URLSearchParams({key: configKey, value}),
          });
          const json: Record<string, any> = await resp.json();
          if (json.errorMessage) throw new Error(json.errorMessage);
        } catch (ex) {
          showTemporaryTooltip(this, ex.toString());
          // Revert the dropdown to the previous value on error
          fomanticQuery(el).dropdown('restore defaults');
        }
      },
    });
  }
}
