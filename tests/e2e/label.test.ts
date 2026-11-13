import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateIssue, apiCreateLabel, apiCreateRepo, randomString} from './utils.ts';

test('create a label in the modal with a color from the picker popup, exclusive scoped labels replace each other in the issue sidebar', async ({page, request}) => {
  const repoName = `e2e-label-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName, autoInit: false});
      await Promise.all([
        apiCreateLabel(request, owner, repoName, 'kind/bug', 'ee0701', {exclusive: true}),
        apiCreateLabel(request, owner, repoName, 'kind/feature', 'ee0701', {exclusive: true}),
        apiCreateIssue(request, {owner, repo: repoName, title: 'Scoped labels'}),
      ]);
    })(),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/labels`);
  await page.getByRole('button', {name: 'New Label'}).click();
  const modal = page.getByRole('dialog');
  await modal.getByRole('button', {name: 'Save'}).click();
  await modal.getByPlaceholder('Label name').fill('e2e-label');
  const colorInput = modal.locator('.color-picker-combo input');
  await colorInput.fill('');
  await page.locator('hex-color-picker').click({position: {x: 10, y: 10}});
  await expect(colorInput).toHaveValue(/^#[0-9a-f]{6}$/);
  await modal.getByRole('button', {name: 'Save'}).click();
  await expect(page.locator('.issue-label-list')).toContainText('e2e-label');
  await page.goto(`/${owner}/${repoName}/issues/1`);
  const trigger = page.getByRole('button', {name: 'Labels'});
  await trigger.click();
  await page.getByRole('menuitem').filter({hasText: 'bug'}).click();
  await page.getByRole('menuitem').filter({hasText: 'feature'}).click();
  await trigger.click();
  await expect(page.locator('.issue-sidebar-combo', {has: trigger}).locator('.labels-list').getByRole('link')).toHaveText([/feature/]);
});
