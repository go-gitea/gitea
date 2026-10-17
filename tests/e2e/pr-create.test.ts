import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiCreateFiles, apiCreatePR, randomString} from './utils.ts';

test('create a pull request from the compare page and edit its title', async ({page, request}) => {
  const repoName = `e2e-pr-create-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await apiCreateRepo(request, {name: repoName});
  await Promise.all([
    apiCreateFiles(request, owner, repoName, [{path: 'feat.txt', content: 'feature content\n'}], {branch: 'main', newBranch: 'feat'}),
    login(page),
  ]);
  // expand=1 renders the PR form directly, skipping the "New Pull Request" toggle click
  await page.goto(`/${owner}/${repoName}/compare/main...feat?expand=1`);

  const title = `e2e-pr-${randomString(8)}`;
  await page.getByPlaceholder('Title').fill(title);
  await page.getByRole('button', {name: 'Create Pull Request'}).click();

  await page.waitForURL(new RegExp(`/${owner}/${repoName}/pulls/\\d+$`), {waitUntil: 'domcontentloaded'});
  await expect(page.getByRole('heading', {name: title})).toBeVisible();

  const titleEditor = page.locator('#issue-title-editor');
  await page.locator('#issue-title-display').getByRole('button', {name: 'Edit'}).click();
  await titleEditor.getByRole('textbox').fill('edited title');
  await titleEditor.getByRole('button', {name: 'Save'}).click();
  await expect(page.getByRole('heading', {name: 'edited title'})).toBeVisible();
});

test('change the target branch of a pull request', async ({page, request}) => {
  const repoName = `e2e-pr-target-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  const createPR = (async () => {
    await apiCreateRepo(request, {name: repoName});
    await Promise.all([
      apiCreateFiles(request, owner, repoName, [{path: 'feat.txt', content: 'feature content\n'}], {branch: 'main', newBranch: 'feat'}),
      apiCreateFiles(request, owner, repoName, [{path: 'release.txt', content: 'release content\n'}], {branch: 'main', newBranch: 'release'}),
    ]);
    return apiCreatePR(request, owner, repoName, 'feat', 'main', 'title');
  })();
  const [index] = await Promise.all([createPR, login(page)]);
  await page.goto(`/${owner}/${repoName}/pulls/${index}`, {waitUntil: 'domcontentloaded'});

  await page.locator('#issue-title-display').getByRole('button', {name: 'Edit'}).click();
  await page.locator('#pull-target-branch').click();
  await page.locator('#branch-select').getByText(`${owner}:release`).click();
  await page.locator('#issue-title-editor').getByRole('button', {name: 'Save'}).click();
  await expect(page.locator('#pull-desc-display')).toContainText('into release');
});
