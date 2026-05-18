import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {apiCreateFile, apiCreateRepo, randomString} from './utils.ts';

test('code-copy on fenced and indented blocks', async ({browserName, page, request}) => {
  const owner = env.GITEA_TEST_E2E_USER;
  const repoName = `e2e-code-copy-${randomString(8)}`;
  await apiCreateRepo(request, {name: repoName, autoInit: false});

  const readme = [
    '# code copy test',
    '',
    '```',
    'fenced content',
    '```',
    '',
    '    indented content',
    '',
  ].join('\n');
  await apiCreateFile(request, owner, repoName, 'README.md', readme, {newBranch: 'main'});

  await page.goto(`/${owner}/${repoName}`);
  const blocks = page.locator('#readme .code-block-container');
  await expect(blocks).toHaveCount(2);
  await expect(blocks.locator('.code-copy')).toHaveCount(2);

  if (browserName !== 'chromium') return; // clipboard read permission is chromium-only

  for (const [idx, expected] of [[0, 'fenced content'], [1, 'indented content']] as const) {
    const block = blocks.nth(idx);
    await block.hover();
    await block.locator('.code-copy').click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(expected);
  }
});
