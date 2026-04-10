import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {apiCreateRepo, apiDeleteRepo, apiHeaders, baseUrl, login, randomString} from './utils.ts';

function generateLines(count: number): string {
  return Array.from({length: count}, (_, idx) => `line ${idx + 1}`).join('\n');
}

test('diff section expand', async ({page, request}) => {
  const repoName = `e2e-diff-expand-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  const headers = apiHeaders();
  await Promise.all([apiCreateRepo(request, {name: repoName}), login(page)]);
  try {
    // create a 40-line file on main, then modify line 20 on a branch to produce collapsed sections
    const originalContent = generateLines(40);
    const createResp = await request.post(`${baseUrl()}/api/v1/repos/${owner}/${repoName}/contents/testfile.txt`, {
      headers, data: {content: globalThis.btoa(originalContent)},
    });
    const modifiedLines = originalContent.split('\n');
    modifiedLines[19] = 'line 20 modified';
    await request.put(`${baseUrl()}/api/v1/repos/${owner}/${repoName}/contents/testfile.txt`, {
      headers,
      data: {
        content: globalThis.btoa(modifiedLines.join('\n')),
        sha: (await createResp.json()).content.sha,
        new_branch: 'diff-test',
      },
    });
    await page.goto(`/${owner}/${repoName}/compare/main...diff-test`);
    const diffBox = page.locator('.diff-file-box').first();
    const diffTable = diffBox.locator('.code-diff');
    await expect(diffTable.locator('[data-line-num="1"]')).toHaveCount(0);
    await expect(diffTable.locator('[data-line-num="40"]')).toHaveCount(0);
    const expandAllBtn = diffBox.locator('[data-global-click="onDiffExpandAll"]');
    const expandButtons = diffBox.locator('.code-expander-button');
    // expand all
    await expandAllBtn.click();
    await expect(expandButtons).toHaveCount(0);
    await expect(diffTable.locator('[data-line-num="1"]').first()).toBeVisible();
    await expect(diffTable.locator('[data-line-num="40"]').first()).toBeVisible();
    await expect(expandAllBtn.locator('.octicon-fold')).toBeVisible();
    // collapse restores original state
    await expandAllBtn.click();
    await expect(expandButtons.first()).toBeVisible();
    await expect(diffTable.locator('[data-line-num="1"]')).toHaveCount(0);
    await expect(diffTable.locator('[data-line-num="40"]')).toHaveCount(0);
    await expect(expandAllBtn.locator('.octicon-unfold')).toBeVisible();
    // single section expand
    await expandButtons.first().evaluate((el: HTMLElement) => el.click());
    await expect(diffTable.locator('[data-line-num="1"]').first()).toBeVisible();
    // expand-all after partial manual expansion
    await expandAllBtn.click();
    await expect(expandButtons).toHaveCount(0);
    await expect(diffTable.locator('[data-line-num="40"]').first()).toBeVisible();
    await expect(expandAllBtn.locator('.octicon-fold')).toBeVisible();
    // collapse restores to before any expansion
    await expandAllBtn.click();
    await expect(expandButtons.first()).toBeVisible();
    await expect(diffTable.locator('[data-line-num="1"]')).toHaveCount(0);
    await expect(diffTable.locator('[data-line-num="40"]')).toHaveCount(0);
    await expect(expandAllBtn.locator('.octicon-unfold')).toBeVisible();
    // manual full expansion auto-flips button to collapse
    while (await expandButtons.count() > 0) {
      const countBefore = await expandButtons.count();
      await expandButtons.first().evaluate((el: HTMLElement) => el.click());
      await expect(expandButtons).not.toHaveCount(countBefore);
    }
    await expect(expandAllBtn.locator('.octicon-fold')).toBeVisible();
    // collapse after manual full expansion
    await expandAllBtn.click();
    await expect(expandButtons.first()).toBeVisible();
    await expect(expandAllBtn.locator('.octicon-unfold')).toBeVisible();
  } finally {
    await apiDeleteRepo(request, owner, repoName);
  }
});
