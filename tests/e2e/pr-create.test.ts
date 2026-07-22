import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiCreateFile, randomString} from './utils.ts';

test('create a pull request from the compare page', async ({page, request}) => {
  const repoName = `e2e-pr-create-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await apiCreateRepo(request, {name: repoName});
  await Promise.all([
    apiCreateFile(request, owner, repoName, 'feat.txt', 'feature content\n', {branch: 'main', newBranch: 'feat'}),
    login(page),
  ]);
  // expand=1 renders the PR form directly, skipping the "New Pull Request" toggle click
  await page.goto(`/${owner}/${repoName}/compare/main...feat?expand=1`);

  const title = `e2e-pr-${randomString(8)}`;
  await page.getByPlaceholder('Title').fill(title);
  await page.getByRole('button', {name: 'Create Pull Request'}).click();

  // commit, not full load: the PR title heading is server-rendered, so the assertion can resolve before the heavy diff/timeline finishes
  await page.waitForURL(new RegExp(`/${owner}/${repoName}/pulls/\\d+$`), {waitUntil: 'commit'});
  await expect(page.getByRole('heading', {name: title})).toBeVisible();
});
