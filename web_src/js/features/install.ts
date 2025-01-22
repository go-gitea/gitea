import {hideElem, showElem} from '../utils/dom.ts';
import {GET} from '../modules/fetch.ts';

export function initInstall() {
  const page = document.querySelector('.page-content.install');
  if (!page) {
    return;
  }
  if (page.classList.contains('post-install')) {
    initPostInstall();
  } else {
    initPreInstall();
  }
}

function initPreInstall() {
  const defaultDbUser = 'gitea';
  const defaultDbName = 'gitea';

  const defaultDbHosts: Record<string, string> = {
    mysql: '127.0.0.1:3306',
    postgres: '127.0.0.1:5432',
    mssql: '127.0.0.1:1433',
  };

  const dbHost = document.querySelector<HTMLInputElement>('#db_host');
  const dbUser = document.querySelector<HTMLInputElement>('#db_user');
  const dbName = document.querySelector<HTMLInputElement>('#db_name');

  // Database type change detection.
  document.querySelector<HTMLInputElement>('#db_type').addEventListener('change', function () {
    const dbType = this.value;
    hideElem('div[data-db-setting-for]');
    showElem(`div[data-db-setting-for=${dbType}]`);

    if (dbType !== 'sqlite3') {
      // for most remote database servers
      showElem('div[data-db-setting-for=common-host]');
      const lastDbHost = dbHost.value;
      const isDbHostDefault = !lastDbHost || Object.values(defaultDbHosts).includes(lastDbHost);
      if (isDbHostDefault) {
        dbHost.value = defaultDbHosts[dbType] ?? '';
      }
      if (!dbUser.value && !dbName.value) {
        dbUser.value = defaultDbUser;
        dbName.value = defaultDbName;
      }
    } // else: for SQLite3, the default path is always prepared by backend code (setting)
  });
  document.querySelector('#db_type').dispatchEvent(new Event('change'));

  const appUrl = document.querySelector<HTMLInputElement>('#app_url');
  if (appUrl.value.includes('://localhost')) {
    appUrl.value = window.location.href;
  }

  const domain = document.querySelector<HTMLInputElement>('#domain');
  if (domain.value.trim() === 'localhost') {
    domain.value = window.location.hostname;
  }

  // TODO: better handling of exclusive relations.
  document.querySelector<HTMLInputElement>('#offline-mode input').addEventListener('change', function () {
    if (this.checked) {
      document.querySelector<HTMLInputElement>('#disable-gravatar input').checked = true;
      document.querySelector<HTMLInputElement>('#federated-avatar-lookup input').checked = false;
    }
  });
  document.querySelector<HTMLInputElement>('#disable-gravatar input').addEventListener('change', function () {
    if (this.checked) {
      document.querySelector<HTMLInputElement>('#federated-avatar-lookup input').checked = false;
    } else {
      document.querySelector<HTMLInputElement>('#offline-mode input').checked = false;
    }
  });
  document.querySelector<HTMLInputElement>('#federated-avatar-lookup input').addEventListener('change', function () {
    if (this.checked) {
      document.querySelector<HTMLInputElement>('#disable-gravatar input').checked = false;
      document.querySelector<HTMLInputElement>('#offline-mode input').checked = false;
    }
  });
  document.querySelector<HTMLInputElement>('#enable-openid-signin input').addEventListener('change', function () {
    if (this.checked) {
      if (!document.querySelector<HTMLInputElement>('#disable-registration input').checked) {
        document.querySelector<HTMLInputElement>('#enable-openid-signup input').checked = true;
      }
    } else {
      document.querySelector<HTMLInputElement>('#enable-openid-signup input').checked = false;
    }
  });
  document.querySelector<HTMLInputElement>('#disable-registration input').addEventListener('change', function () {
    if (this.checked) {
      document.querySelector<HTMLInputElement>('#enable-captcha input').checked = false;
      document.querySelector<HTMLInputElement>('#enable-openid-signup input').checked = false;
    } else {
      document.querySelector<HTMLInputElement>('#enable-openid-signup input').checked = true;
    }
  });
  document.querySelector<HTMLInputElement>('#enable-captcha input').addEventListener('change', function () {
    if (this.checked) {
      document.querySelector<HTMLInputElement>('#disable-registration input').checked = false;
    }
  });
}

function initPostInstall() {
  const el = document.querySelector('#goto-user-login');
  if (!el) return;

  const targetUrl = el.getAttribute('href');
  let tid = setInterval(async () => {
    try {
      const resp = await GET(targetUrl);
      if (tid && resp.status === 200) {
        clearInterval(tid);
        tid = null;
        window.location.href = targetUrl;
      }
    } catch {}
  }, 1000);
}
