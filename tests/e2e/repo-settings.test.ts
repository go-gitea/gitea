import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {apiCreateBranchProtection, apiCreateRepo, login, randomString} from './utils.ts';

test('branch protection rule keeps its push allowlist and dragged priority', async ({page, request}) => {
  const owner = env.GITEA_TEST_E2E_USER;
  const repo = `e2e-protection-${randomString(8)}`;
  await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repo, autoInit: false});
      await apiCreateBranchProtection(request, owner, repo, 'release/stable');
    })(),
    login(page),
  ]);
  await page.goto(`/${owner}/${repo}/settings/branches/edit`);
  const ruleNameInput = page.getByRole('textbox', {name: /^Protected Branch Name Pattern/});
  const allowlistRadio = page.getByRole('radio', {name: 'Allowlist Restricted Push', exact: true});
  await ruleNameInput.fill('release/**');
  await allowlistRadio.check();
  await page.locator('#whitelist_box').getByRole('combobox').click();
  await page.getByRole('option', {name: owner}).click();
  await page.getByRole('button', {name: 'Save Rule', exact: true}).click();
  await page.waitForURL(`/${owner}/${repo}/settings/branches`);

  const rules = page.locator('#protected-branches-list > .item');
  await expect(rules).toHaveText([/release\/stable/, /release\/\*\*/]);
  const rule = rules.filter({hasText: 'release/**'});
  const handle = rule.locator('.drag-handle');
  const saved = page.waitForResponse('**/settings/branches/priority');
  await handle.hover();
  await page.mouse.down();
  await handle.hover({position: {x: 2, y: 2}});
  await expect(rule).toHaveClass(/card-ghost/);
  await rules.first().hover({position: {x: 10, y: 10}});
  await expect(rules.first()).toContainText('release/**');
  await page.mouse.up();
  await saved;
  await page.reload();
  await expect(rules).toHaveText([/release\/\*\*/, /release\/stable/]);

  await rule.getByRole('link', {name: 'Edit', exact: true}).click();
  await expect(ruleNameInput).toHaveValue('release/**');
  await expect(allowlistRadio).toBeChecked();
  await expect(page.locator('#whitelist_box .dropdown > .label')).toHaveText(owner);
});
