import {hideElem, showElem} from '../utils/dom.js';
import {GET} from '../modules/fetch.js';

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

  const defaultDbHosts = {
    mysql: '127.0.0.1:3306',
    postgres: '127.0.0.1:5432',
    mssql: '127.0.0.1:1433',
  };

  const dbHost = document.querySelector('#db_host');
  const dbUser = document.querySelector('#db_user');
  const dbName = document.querySelector('#db_name');

  // Database type change detection.
  document.querySelector('#db_type').addEventListener('change', function () {
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

  const appUrl = document.querySelector('#app_url');
  if (appUrl.value.includes('://localhost')) {
    appUrl.value = window.location.href;
  }

  const domain = document.querySelector('#domain');
  if (domain.value.trim() === 'localhost') {
    domain.value = window.location.hostname;
  }

  // TODO: better handling of exclusive relations.
  document.querySelector('#offline-mode input').addEventListener('change', function () {
    if (this.checked) {
      document.querySelector('#disable-gravatar input').checked = true;
      document.querySelector('#federated-avatar-lookup input').checked = false;
    }
  });
  document.querySelector('#disable-gravatar input').addEventListener('change', function () {
    if (this.checked) {
      document.querySelector('#federated-avatar-lookup input').checked = false;
    } else {
      document.querySelector('#offline-mode input').checked = false;
    }
  });
  document.querySelector('#federated-avatar-lookup input').addEventListener('change', function () {
    if (this.checked) {
      document.querySelector('#disable-gravatar input').checked = false;
      document.querySelector('#offline-mode input').checked = false;
    }
  });
  document.querySelector('#enable-openid-signin input').addEventListener('change', function () {
    if (this.checked) {
      if (!document.querySelector('#disable-registration input').checked) {
        document.querySelector('#enable-openid-signup input').checked = true;
      }
    } else {
      document.querySelector('#enable-openid-signup input').checked = false;
    }
  });
  document.querySelector('#disable-registration input').addEventListener('change', function () {
    if (this.checked) {
      document.querySelector('#enable-captcha input').checked = false;
      document.querySelector('#enable-openid-signup input').checked = false;
    } else {
      document.querySelector('#enable-openid-signup input').checked = true;
    }
  });
  document.querySelector('#enable-captcha input').addEventListener('change', function () {
    if (this.checked) {
      document.querySelector('#disable-registration input').checked = false;
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
