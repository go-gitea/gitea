import {chromium} from '@playwright/test';

const OUT = '/private/tmp/claude-501/-Users-nicolas-Github-go-gitea-plain-gitea/af3f0158-4e96-4069-94d0-54d238709dae/scratchpad/shots';
const BASE = 'http://localhost:3000';

// the pages a restyle actually has to survive: the shell, the component gallery,
// the big repo surfaces, and the settings/admin forms
const PAGES: Array<[string, string]> = [
  ['dashboard', '/'],
  ['devtest-ui', '/devtest/gitea-ui'],
  ['devtest-list', '/devtest'],
  ['devtest-buttons', '/devtest/global-button'],
  ['devtest-forms', '/devtest/form-fields'],
  ['repo', '/admin/test-repo'],
  ['pulls', '/admin/test-repo/pulls'],
  ['pr-view', '/admin/test-repo/pulls/1'],
  ['pr-files', '/admin/test-repo/pulls/1/files'],
  ['commits', '/admin/test-repo/commits/branch/main'],
  ['releases', '/admin/test-repo/releases'],
  ['repo-settings', '/admin/test-repo/settings'],
  ['user-settings', '/user/settings'],
  ['appearance', '/user/settings/appearance'],
  ['admin-panel', '/-/admin'],
  ['admin-users', '/-/admin/users'],
  ['explore', '/explore/repos'],
  ['profile', '/admin'],
  ['new-repo', '/repo/create'],
];

const themes = (process.env.THEMES || 'atlas-dark').split(',');
const browser = await chromium.launch();
let bad = 0;

for (const theme of themes) {
  const ctx = await browser.newContext({viewport: {width: 1440, height: 1000}});
  await ctx.addCookies([{name: 'gitea_theme', value: theme, url: BASE}]);
  const page = await ctx.newPage();

  // sign in
  await page.goto(`${BASE}/user/login`, {waitUntil: 'load'});
  await page.fill('input[name="user_name"]', 'admin');
  await page.fill('input[name="password"]', 'AtlasDev!2026');
  await page.click('form button.ui.primary.button');
  await page.waitForURL((u) => !u.pathname.includes('/user/login'), {timeout: 15000});

  // a signed-in user's theme comes from the DB, not the cookie, so set it for real
  await page.goto(`${BASE}/user/settings/appearance`, {waitUntil: 'load'});
  await page.evaluate((t) => {
    const form = document.querySelector('form[action$="/theme"]') as HTMLFormElement;
    (form.querySelector('input[name="theme"]') as HTMLInputElement).value = t;
    form.submit();
  }, theme);
  await page.waitForLoadState('load');
  const active = await page.getAttribute('html', 'data-theme');
  if (active !== theme) throw new Error(`theme did not apply: wanted ${theme}, got ${active}`);

  for (const [name, path] of PAGES) {
    const errors: string[] = [];
    const onErr = (e: Error) => errors.push(String(e));
    page.on('pageerror', onErr);
    const res = await page.goto(BASE + path, {waitUntil: 'load'});
    await page.waitForTimeout(500);
    await page.screenshot({path: `${OUT}/${theme}-${name}.png`, fullPage: false});
    page.off('pageerror', onErr);

    const status = res?.status() ?? 0;
    const flag = status >= 400 || errors.length ? ' <<<< PROBLEM' : '';
    if (flag) bad++;
    console.log(`${theme.padEnd(11)} ${name.padEnd(16)} http=${status} js=${errors.length}${flag}`);
    if (errors.length) console.log(`             ${errors.join(' | ')}`);
  }
  await ctx.close();
}

await browser.close();
console.log(bad ? `\n${bad} page(s) with problems` : '\nall pages clean');
