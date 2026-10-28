import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {apiCreateFiles, apiCreatePR, apiCreateRepo, login, randomString} from './utils.ts';

test('a copied pull request diff pastes as text, not as a markdown table', async ({page, request}) => {
  const repoName = `e2e-pr-diff-copy-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  const createPR = (async () => {
    await apiCreateRepo(request, {name: repoName});
    await apiCreateFiles(request, owner, repoName, [{path: 'copy.txt', content: 'one | two\n'}], {branch: 'main', newBranch: 'feat'});
    return apiCreatePR(request, owner, repoName, 'feat', 'main', 'title');
  })();
  const [index] = await Promise.all([createPR, login(page)]);
  await page.goto(`/${owner}/${repoName}/pulls/${index}/files`);

  await page.getByRole('main').selectText();
  await page.keyboard.press('ControlOrMeta+KeyC');
  await page.getByRole('button', {name: 'Review', exact: true}).click();
  const comment = page.getByRole('textbox', {name: 'Review comment'});
  await comment.press('ControlOrMeta+KeyV');
  await expect(comment).toHaveValue(/one \| two/);
  await expect(comment).not.toHaveValue(/one \\\| two/);
});
