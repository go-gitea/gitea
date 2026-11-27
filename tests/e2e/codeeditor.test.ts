import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {login, apiCreateFiles, apiCreateRepo, randomString} from './utils.ts';

test('codeeditor textarea updates correctly', async ({page, request}) => {
  const repoName = `e2e-codeeditor-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName});
      await apiCreateFiles(request, owner, repoName, [{path: '.editorconfig', content: '[*]\nindent_size = 2\n'}], {newBranch: 'feature'});
    })(),
    login(page),
  ]);
  await page.goto(`/${owner}/${repoName}/_new/feature`);
  await page.locator('.cm-content[role="textbox"]').click();
  await page.keyboard.press('Tab');
  await expect(page.locator('textarea[name="content"]')).toHaveValue('  ');
});

test('preview and propose a markdown edit', async ({page, request}) => {
  const repoName = `e2e-editor-preview-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  const repoPath = `/${owner}/${repoName}`;
  const filePath = 'docs/guide.md';
  const unchangedLine = 'Original guide context.';
  const proposedHeading = 'Proposed guide heading';
  await Promise.all([
    (async () => {
      await apiCreateRepo(request, {name: repoName, autoInit: false});
      await apiCreateFiles(request, owner, repoName, [{path: filePath, content: `${unchangedLine}\n`}]);
    })(),
    login(page),
  ]);

  await page.goto(`${repoPath}/src/branch/main/${filePath}`);
  await page.getByRole('link', {name: 'Edit File', exact: true}).click();
  await page.waitForURL(`${repoPath}/_edit/main/${filePath}`);

  const main = page.getByRole('main');
  const commitButton = main.getByRole('button', {name: 'Commit Changes', exact: true});
  await expect(commitButton).toBeDisabled();
  await main.getByRole('textbox').filter({hasText: unchangedLine}).fill(
    `${unchangedLine}\n\n### ${proposedHeading}\n\n[Notes](notes.md)\n`,
  );
  await expect(commitButton).toBeEnabled();

  await main.getByText('Preview', {exact: true}).click();
  await expect(main.getByRole('heading', {name: proposedHeading, exact: true})).toBeVisible();
  await expect(main.getByRole('link', {name: 'Notes', exact: true})).toHaveJSProperty(
    'href', new URL(`${repoPath}/src/branch/main/docs/notes.md`, page.url()).href,
  );

  await main.getByText('Preview Changes', {exact: true}).click();
  const addedRow = main.getByRole('row', {name: `3 + ### ${proposedHeading}`, exact: true});
  const unchangedRow = main.getByRole('row', {name: `1 1 ${unchangedLine}`, exact: true});
  await expect(addedRow).toBeVisible();
  await expect(unchangedRow).toBeVisible();

  await main.getByRole('radio', {
    name: 'Create a new branch for this commit and start a pull request.', exact: true,
  }).check();
  await main.getByPlaceholder('New branch name…', {exact: true}).fill('docs-preview');
  await main.getByRole('button', {name: 'Propose file change', exact: true}).click();
  await page.waitForURL(`${repoPath}/compare/main...docs-preview`);
  await expect(addedRow).toBeVisible();
  await expect(unchangedRow).toBeVisible();

  await main.getByRole('button', {name: 'New Pull Request', exact: true}).click();
  await expect(main.getByPlaceholder('Title', {exact: true})).toBeVisible();
});
