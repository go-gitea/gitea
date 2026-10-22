import {test, expect, type Page, type APIRequestContext, type Route} from '@playwright/test';
import {apiCreateFiles, apiCreatePR, apiCreateRepo, apiCreateUser, apiUserHeaders, loginUser, randomString} from './utils.ts';

const fileLineCount = 120;
const hiddenLineCount = 99; // the lines the diff does not show around its three hunks

// a PR whose file has a gap before, between and after the hunks
async function createDiffPull(page: Page, request: APIRequestContext, style = 'unified') {
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
  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files?style=${style}`);
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

  // a single arrow expands one chunk, and expanding all on top of it must not duplicate lines
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

test('expanding all leaves the rows an arrow already expanded in place', async ({page, request}) => {
  const {rows, arrows, toggle} = await createDiffPull(page, request);
  const collapsedCount = await rows.count();

  await arrows.first().click();
  await expect(rows).toHaveCount(collapsedCount + 16);
  // mark the rows of the gap the arrow fully expanded: expanding all must not rebuild them
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
  // an arrow and expanding all ask the same way, and the arrow's request has already finished
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

test('a line expanded in split view comments on the right side', async ({page, request}) => {
  const {arrows} = await createDiffPull(page, request, 'split');
  await arrows.first().click();
  await expect(page.locator('#diff-file-boxes tr[data-expand-gap]').first()).toBeVisible();

  // a line the diff shows takes a comment on whichever side it was clicked
  await expect(page.locator('#diff-file-boxes tr:not([data-expand-gap]) td.lines-code-old .add-code-comment').first()).toHaveAttribute('data-side', 'left');
  // a expanded line comments on the right from either cell, because FillHiddenCommentIDsForDiffLine
  // counts only right side comments and a left side one would be lost once the gap is closed again
  await expect(page.locator('#diff-file-boxes tr[data-expand-gap] td.lines-code-old .add-code-comment').first()).toHaveAttribute('data-side', 'right');
});

test('reopening a file it already expanded costs no request', async ({page, request}) => {
  const {rows, toggle} = await createDiffPull(page, request);
  const collapsedCount = await rows.count();
  let requests = 0;
  await page.route((url) => url.searchParams.has('gap'), async (route) => { requests++; await route.continue() });

  await toggle.click();
  await expect(rows).toHaveCount(collapsedCount + hiddenLineCount);
  await toggle.click();
  await expect(rows).toHaveCount(collapsedCount);
  expect(requests).toEqual(1);

  // the rows are kept, so opening it again puts them back without asking for them
  await toggle.click();
  await expect(rows).toHaveCount(collapsedCount + hiddenLineCount);
  expect(requests).toEqual(1);
});

test('expanding a gap from both ends at once keeps its lines in order', async ({page, request}) => {
  const {rows, arrows} = await createDiffPull(page, request);
  const collapsedCount = await rows.count();
  const held: Array<{route: Route; up: boolean}> = [];
  await page.route((url) => url.searchParams.has('gap'), async (route) => {
    if (held.length === 2) return route.continue();
    held.push({route, up: new URL(route.request().url()).searchParams.get('gap')!.endsWith(',up')});
  });

  // the middle gap offers both arrows, and both are clicked before either answers
  await arrows.nth(1).click();
  await arrows.nth(2).click();
  await expect.poll(() => held.length).toBe(2);
  // the "up" chunk lands first, so the "down" one asked for a gap that is no longer there
  const [up, down] = held.toSorted((a, b) => Number(b.up) - Number(a.up));
  await up.route.continue();
  await expect(rows).toHaveCount(collapsedCount + 20);
  const downResponse = page.waitForResponse((resp) => resp.request() === down.route.request());
  await down.route.continue();
  await downResponse;

  await arrows.nth(1).click(); // the rest of the gap, above the chunk that was expanded upwards
  await expect(rows).toHaveCount(collapsedCount + 33);
  const lineNums = await page.locator('#diff-file-boxes .lines-num-new[data-line-num]:not([data-line-num=""])').evaluateAll(
    (els) => els.map((el) => Number(el.getAttribute('data-line-num'))),
  );
  expect(lineNums).toEqual(lineNums.toSorted((a, b) => a - b));
  expect(new Set(lineNums).size).toEqual(lineNums.length);
});

test('loading a suppressed file keeps its header and brings the actions that need its body', async ({page, request}) => {
  const user = `de-${randomString(8)}`;
  await apiCreateUser(request, user);
  const headers = apiUserHeaders(user);
  const repo = `e2e-diffload-${randomString(8)}`;
  await apiCreateRepo(request, {name: repo, headers});
  // more changed lines than a diff file renders, with unchanged lines around them to expand
  const lines = Array.from({length: 1100}, (_, i) => `line ${i + 1}`);
  await apiCreateFiles(request, user, repo, [{path: 'big.txt', content: `${lines.join('\n')}\n`}], {branch: 'main', headers});
  for (let i = 50; i < 1060; i++) lines[i] = `changed ${i + 1}`;
  await apiCreateFiles(request, user, repo, [{path: 'big.txt', content: `${lines.join('\n')}\n`, operation: 'update'}], {branch: 'main', newBranch: 'edit', headers});
  const prIndex = await apiCreatePR(request, user, repo, 'edit', 'main', 'diff load test', {headers});
  await loginUser(page, user);
  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files`);

  const fileBox = page.locator('#diff-file-boxes .diff-file-box');
  const header = fileBox.locator('.diff-file-header');
  await header.evaluate((el) => el.setAttribute('data-marked', '1'));
  await expect(fileBox.locator('.diff-expand-lines-button')).toHaveCount(0);
  await fileBox.getByText('Load diff').click();

  await expect(fileBox.locator('.diff-expand-lines-button')).toBeVisible();
  await expect(header).toHaveAttribute('data-marked', '1'); // the same header, not a fresh one
  await header.locator('[data-global-init="initDiffHeaderPopupMenu"]').click();
  await expect(page.getByRole('button', {name: 'Unescape', exact: true})).toBeVisible();
});
