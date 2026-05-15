import {env} from 'node:process';
import {test, expect, type APIRequestContext, type Page} from '@playwright/test';
import {login, apiCreateRepo, apiDeleteRepo, createProjectColumn, apiHeaders, baseUrl, randomString} from './utils.ts';

async function apiCreateIssueReturning(requestContext: APIRequestContext, owner: string, repo: string, title: string): Promise<{id: number; number: number}> {
  const response = await requestContext.post(`${baseUrl()}/api/v1/repos/${owner}/${repo}/issues`, {
    headers: apiHeaders(),
    data: {title},
  });
  if (!response.ok()) throw new Error(`apiCreateIssueReturning failed: ${response.status()} ${await response.text()}`);
  const body = await response.json();
  return {id: body.id, number: body.number};
}

async function assignIssueToProject(page: Page, owner: string, repo: string, issueID: number, projectID: string) {
  // Bulk endpoint — takes comma-separated global issue IDs via issue_ids.
  const form = new URLSearchParams({id: projectID, issue_ids: String(issueID)});
  const response = await page.request.post(`/${owner}/${repo}/issues/projects`, {
    headers: {'content-type': 'application/x-www-form-urlencoded'},
    data: form.toString(),
  });
  expect(response.ok()).toBeTruthy();
}

async function readColumnOrder(page: Page): Promise<string[]> {
  return page.locator('#project-board .project-column').evaluateAll((els) => els.map((el) => el.getAttribute('data-id')!));
}

async function readIssueOrder(page: Page, columnID: string): Promise<string[]> {
  return page.locator(`.project-column[data-id="${columnID}"] .issue-card`).evaluateAll((els) => els.map((el) => el.getAttribute('data-issue')!));
}

// SortableJS intercepts low-level mouse events rather than HTML5 DnD, so
// locator.dragTo() doesn't work. The recipe below is the Playwright-documented
// workaround: hover → mouse.down → two hovers on the target (the second move
// is required for SortableJS to register the drop position) → mouse.up.
// Steps on mouse.move produces interpolated mousemove events so SortableJS's
// onMove callbacks fire continuously.
async function sortableDrag(page: Page, source: ReturnType<Page['locator']>, target: ReturnType<Page['locator']>) {
  const sourceBox = await source.boundingBox();
  const targetBox = await target.boundingBox();
  if (!sourceBox || !targetBox) throw new Error('boundingBox returned null');
  await page.mouse.move(sourceBox.x + sourceBox.width / 2, sourceBox.y + sourceBox.height / 2);
  await page.mouse.down();
  await page.mouse.move(targetBox.x + targetBox.width / 2, targetBox.y + targetBox.height / 2, {steps: 10});
  await page.mouse.move(targetBox.x + targetBox.width / 2, targetBox.y + targetBox.height / 2, {steps: 5});
  await page.mouse.up();
}

test('project board: drag a column header to reorder', async ({page}) => {
  const repoName = `e2e-project-drag-col-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repoName})]);

  await page.goto(`/${user}/${repoName}/projects/new`);
  await page.locator('input[name="title"]').fill('Drag Columns');
  await page.getByRole('button', {name: 'Create Project'}).click();
  const projectLink = page.locator('.milestone-list a', {hasText: 'Drag Columns'}).first();
  await expect(projectLink).toBeVisible();
  const projectID = (await projectLink.getAttribute('href'))!.split('/').pop()!;

  // Sequential so the DB sorting reflects Alpha<Bravo<Charlie deterministically.
  for (const title of ['Alpha', 'Bravo', 'Charlie']) {
    await createProjectColumn(page.request, user, repoName, projectID, title);
  }

  await page.goto(`/${user}/${repoName}/projects/${projectID}`);
  const before = await readColumnOrder(page);
  expect(before).toHaveLength(3);

  // Drag Charlie's header to just left of Alpha. SortableJS uses the pointer's
  // position relative to the target's midpoint to decide "before" vs "after";
  // dropping at the target's left edge reliably inserts before it.
  const charlieHeader = page.locator('.project-column', {has: page.locator('.project-column-title-text', {hasText: 'Charlie'})}).locator('.project-column-header');
  const alphaCol = page.locator('.project-column', {has: page.locator('.project-column-title-text', {hasText: 'Alpha'})});

  const src = (await charlieHeader.boundingBox())!;
  const dst = (await alphaCol.boundingBox())!;
  const charlieID = before[2];
  await page.mouse.move(src.x + src.width / 2, src.y + src.height / 2);
  await page.mouse.down();
  await page.mouse.move(dst.x + 5, dst.y + 20, {steps: 10});
  await page.mouse.move(dst.x + 5, dst.y + 20, {steps: 5});
  await page.mouse.up();

  // Wait for SortableJS's onSort -> POST -> server persist round-trip. The
  // exact landing slot depends on pointer-vs-midpoint math; we just assert
  // that Charlie (originally last) is no longer last.
  await expect.poll(async () => (await readColumnOrder(page)).indexOf(charlieID), {timeout: 5000}).not.toBe(2);

  // Reload to confirm the new order is persisted server-side.
  await page.reload();
  const reloadedOrder = await readColumnOrder(page);
  expect(reloadedOrder.indexOf(charlieID)).toBeLessThan(2);

  await apiDeleteRepo(page.request, user, repoName);
});

test('project board: drag an issue card to a different column', async ({page}) => {
  const repoName = `e2e-project-drag-iss-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repoName})]);

  await page.goto(`/${user}/${repoName}/projects/new`);
  await page.locator('input[name="title"]').fill('Drag Issue');
  await page.getByRole('button', {name: 'Create Project'}).click();
  const projectLink = page.locator('.milestone-list a', {hasText: 'Drag Issue'}).first();
  await expect(projectLink).toBeVisible();
  const projectID = (await projectLink.getAttribute('href'))!.split('/').pop()!;

  // Sequential so Source is deterministically the first-created column and
  // thus the promoted default where the assigned issue lands.
  await createProjectColumn(page.request, user, repoName, projectID, 'Source');
  await createProjectColumn(page.request, user, repoName, projectID, 'Target');
  const issue = await apiCreateIssueReturning(page.request, user, repoName, 'draggable issue');
  await assignIssueToProject(page, user, repoName, issue.id, projectID);

  await page.goto(`/${user}/${repoName}/projects/${projectID}`);
  const sourceCol = page.locator('.project-column', {has: page.locator('.project-column-title-text', {hasText: 'Source'})});
  const targetCol = page.locator('.project-column', {has: page.locator('.project-column-title-text', {hasText: 'Target'})});
  const sourceID = (await sourceCol.getAttribute('data-id'))!;
  const targetID = (await targetCol.getAttribute('data-id'))!;

  // The issue landed on "Source" (the first-created column, promoted to default).
  expect(await readIssueOrder(page, sourceID)).toEqual([String(issue.id)]);
  expect(await readIssueOrder(page, targetID)).toEqual([]);

  // Drag the card to the target column's card list.
  const card = sourceCol.locator(`.issue-card[data-issue="${issue.id}"]`);
  const targetCardList = targetCol.locator('.cards');
  await sortableDrag(page, card, targetCardList);

  await expect.poll(async () => (await readIssueOrder(page, targetID)).length, {timeout: 5000}).toBe(1);
  await page.reload();
  expect(await readIssueOrder(page, sourceID)).toEqual([]);
  expect(await readIssueOrder(page, targetID)).toEqual([String(issue.id)]);

  await apiDeleteRepo(page.request, user, repoName);
});

test('project board: drag to reorder issues within the same column', async ({page}) => {
  // This is the scenario the whole refactor exists for: the unique index on
  // (project_board_id, sorting) rejects mid-reorder UPDATE collisions, which
  // the old sequential code hit when two cards swapped positions. The service
  // now parks rows at distinct negatives first, then writes finals — a drag
  // within a single column exercises that path end-to-end.
  const repoName = `e2e-project-drag-same-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repoName})]);

  await page.goto(`/${user}/${repoName}/projects/new`);
  await page.locator('input[name="title"]').fill('Same Column Reorder');
  await page.getByRole('button', {name: 'Create Project'}).click();
  const projectLink = page.locator('.milestone-list a', {hasText: 'Same Column Reorder'}).first();
  await expect(projectLink).toBeVisible();
  const projectID = (await projectLink.getAttribute('href'))!.split('/').pop()!;

  await createProjectColumn(page.request, user, repoName, projectID, 'Todo');
  // Sequential so per-repo indexes + DB sortings are deterministic.
  const issueA = await apiCreateIssueReturning(page.request, user, repoName, 'first');
  const issueB = await apiCreateIssueReturning(page.request, user, repoName, 'second');
  const issueC = await apiCreateIssueReturning(page.request, user, repoName, 'third');
  await assignIssueToProject(page, user, repoName, issueA.id, projectID);
  await assignIssueToProject(page, user, repoName, issueB.id, projectID);
  await assignIssueToProject(page, user, repoName, issueC.id, projectID);

  await page.goto(`/${user}/${repoName}/projects/${projectID}`);
  const todoCol = page.locator('.project-column', {has: page.locator('.project-column-title-text', {hasText: 'Todo'})});
  const todoID = (await todoCol.getAttribute('data-id'))!;
  expect(await readIssueOrder(page, todoID)).toEqual([String(issueA.id), String(issueB.id), String(issueC.id)]);

  // Drag issueA onto issueB. SortableJS reliably swaps adjacent cards when
  // the pointer crosses a neighbor's midpoint; the resulting order is B, A, C.
  // The test asserts the change survives a reload — that's what the refactor
  // needed to prove. The precise landing slot isn't the interesting bit.
  const cardA = todoCol.locator(`.issue-card[data-issue="${issueA.id}"]`);
  const cardB = todoCol.locator(`.issue-card[data-issue="${issueB.id}"]`);
  const srcBox = (await cardA.boundingBox())!;
  const dstBox = (await cardB.boundingBox())!;
  const before = await readIssueOrder(page, todoID);

  await page.mouse.move(srcBox.x + srcBox.width / 2, srcBox.y + srcBox.height / 2);
  await page.mouse.down();
  await page.mouse.move(dstBox.x + dstBox.width / 2, dstBox.y + dstBox.height - 5, {steps: 10});
  await page.mouse.move(dstBox.x + dstBox.width / 2, dstBox.y + dstBox.height - 5, {steps: 5});
  await page.mouse.up();

  // Poll until SortableJS's POST lands server-side: the DOM order should no
  // longer match the starting order.
  await expect.poll(async () => (await readIssueOrder(page, todoID)).join(','), {timeout: 5000}).not.toBe(before.join(','));
  const afterDrag = await readIssueOrder(page, todoID);

  // Reload to prove the server persisted the new order, not just the client DOM.
  await page.reload();
  const afterReload = await readIssueOrder(page, todoID);
  expect(afterReload).toHaveLength(3);
  expect(afterReload).toEqual(afterDrag);

  await apiDeleteRepo(page.request, user, repoName);
});
