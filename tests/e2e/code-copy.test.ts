import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {apiCreateFile, apiCreateRepo, randomString} from './utils.ts';

test('code-copy on fenced and indented blocks', async ({page, request}) => {
  const owner = env.GITEA_TEST_E2E_USER;
  const repoName = `e2e-code-copy-${randomString(8)}`;
  await apiCreateRepo(request, {name: repoName, autoInit: false});

  const readme = `# code copy test

\`\`\`
fenced content
\`\`\`

    indented content
`;
  await apiCreateFile(request, owner, repoName, 'README.md', readme, {newBranch: 'main'});

  await page.goto(`/${owner}/${repoName}`);
  const blocks = page.locator('#readme .code-block-container');
  await expect(blocks).toHaveCount(2);

  for (const [index, expected] of (['fenced content', 'indented content'] as const).entries()) {
    const block = blocks.nth(index);
    await block.hover();
    await block.getByRole('button').click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(expected);
  }
});
