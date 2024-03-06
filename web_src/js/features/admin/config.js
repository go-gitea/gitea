import {showTemporaryTooltip} from '../../modules/tippy.js';
import {POST} from '../../modules/fetch.js';

const {appSubUrl} = window.config;

export function initAdminConfigs() {
  const elAdminConfig = document.querySelector('.page-content.admin.config');
  if (!elAdminConfig) return;

  for (const el of elAdminConfig.querySelectorAll('input[type="checkbox"][data-config-dyn-key]')) {
    el.addEventListener('change', async () => {
      try {
        const resp = await POST(`${appSubUrl}/admin/config`, {
          data: new URLSearchParams({key: el.getAttribute('data-config-dyn-key'), value: el.checked}),
        });
        const json = await resp.json();
        if (json.errorMessage) throw new Error(json.errorMessage);
      } catch (ex) {
        showTemporaryTooltip(el, ex.toString());
        el.checked = !el.checked;
      }
    });
  }
}
