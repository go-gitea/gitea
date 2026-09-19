import {test, expect, type Page, type APIRequestContext} from '@playwright/test';
import {apiCreateFiles, apiCreatePR, apiCreateRepo, apiCreateUser, apiUserHeaders, loginUser, randomString} from './utils.ts';

const fileLineCount = 120;
const hiddenLineCount = 99; // the lines the diff does not show around its three hunks

// a PR whose file has a gap before, between and after the hunks
async function createDiffPull(page: Page, request: APIRequestContext) {
  const user = `de-${randomString(8)}`;
  await apiCreateUser(request, user);
  const headers = apiUserHeaders(user);
  const repo = `e2e-diffexpand-${randomString(8)}`;
  await apiCreateRepo(request, {name: repo, headers});

  const lines = Array.from({length: fileLineCount}, (_, i) => `line ${String(i + 1).padStart(3, '0')}: the quick brown fox`);
  await apiCreateFiles(request, user, repo, [{path: 'sample.txt', content: `${lines.join('\n')}\n`}], {branch: 'main', headers});
  for (const i of [19, 59, 99]) lines[i] = `line ${String(i + 1).padStart(3, '0')}: CHANGED`;
  await apiCreateFiles(request, user, repo, [{path: 'sample.txt', content: `${lines.join('\n')}\n`, operation: 'update'}], {branch: 'main', newBranch: 'edit', headers});
  const prIndex = await apiCreatePR(request, user, repo, 'edit', 'main', 'diff expand test', {headers});

  await loginUser(page, user);
  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files`);
  return {
    rows: page.locator('#diff-file-boxes table.chroma tr'),
    arrows: page.locator('#diff-file-boxes .code-expander-button'),
    toggle: page.locator('#diff-file-boxes .diff-expand-lines-button'),
  };
}

test('expand and collapse the hidden lines of a diff file', async ({page, request}) => {
  const {rows, arrows, toggle} = await createDiffPull(page, request);
  const collapsedCount = await rows.count();
  await expect(toggle).toHaveAttribute('aria-label', 'Expand all lines');

  // a single arrow reveals one chunk, and expanding all on top of it must not duplicate lines
  await arrows.first().click();
  await expect(rows).toHaveCount(collapsedCount + 16);

  await toggle.click();
  await expect(page.locator(`#diff-file-boxes .lines-num-new[data-line-num="${fileLineCount}"]`)).toBeVisible();
  await expect(rows).toHaveCount(collapsedCount + hiddenLineCount);
  await expect(toggle).toHaveAttribute('aria-label', 'Collapse non-diff lines');

  await toggle.click();
  await expect(rows).toHaveCount(collapsedCount);
  await expect(page.locator('#diff-file-boxes tr[data-expand-gap]')).toHaveCount(0);
});

test('expanding all leaves the rows an arrow already revealed in place', async ({page, request}) => {
  const {rows, arrows, toggle} = await createDiffPull(page, request);
  const collapsedCount = await rows.count();

  await arrows.first().click();
  await expect(rows).toHaveCount(collapsedCount + 16);
  // mark the rows of the gap the arrow fully revealed: expanding all must not rebuild them
  await page.locator('#diff-file-boxes tr[data-expand-gap]').evaluateAll((els) => {
    for (const el of els) el.setAttribute('data-marked', '1');
  });

  await toggle.click();
  await expect(rows).toHaveCount(collapsedCount + hiddenLineCount);
  await expect(page.locator('#diff-file-boxes tr[data-marked="1"]')).toHaveCount(16);
});

test('the button collapses a file whose gaps were all expanded by hand', async ({page, request}) => {
  const {rows, arrows, toggle} = await createDiffPull(page, request);
  const collapsedCount = await rows.count();

  for (let prev = collapsedCount; await arrows.count(); prev = await rows.count()) {
    await arrows.first().click();
    await expect(rows).not.toHaveCount(prev);
  }
  await expect(rows).toHaveCount(collapsedCount + hiddenLineCount);
  await expect(page.locator('#diff-file-boxes .lines-num-new[data-line-num="1"]')).toBeVisible();
  await expect(page.locator(`#diff-file-boxes .lines-num-new[data-line-num="${fileLineCount}"]`)).toBeVisible();

  // the toggle reads the file, not its own past clicks, so it now offers a collapse rather than a no-op expand
  await expect(toggle).toHaveAttribute('aria-label', 'Collapse non-diff lines');
  await toggle.click();
  await expect(rows).toHaveCount(collapsedCount);
});

test('expanding all keeps an already expanded gap on screen while it loads', async ({page, request}) => {
  const {rows, arrows, toggle} = await createDiffPull(page, request);
  const collapsedCount = await rows.count();

  await arrows.first().click();
  await expect(rows).toHaveCount(collapsedCount + 16);

  const held = Promise.withResolvers<void>();
  // expanding all names the gaps it wants, which is what distinguishes it from a single arrow
  await page.route((url) => url.searchParams.has('gap'), async (route) => {
    await held.promise;
    await route.continue();
  });

  const expandAllRequest = page.waitForRequest((req) => new URL(req.url()).searchParams.has('gap'));
  await toggle.click();
  await expandAllRequest;
  await expect(rows).toHaveCount(collapsedCount + 16); // the already expanded gap must not fold away while waiting

  held.resolve();
  await expect(rows).toHaveCount(collapsedCount + hiddenLineCount);
});
