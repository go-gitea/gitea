import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {
  apiCreateIssue, apiCreateProject, apiCreateRepo, apiDeleteRepo, apiHeaders, baseUrl, createProjectColumn, login, randomString,
} from './utils.ts';

const owner = env.GITEA_TEST_E2E_USER;

test('project board: drag moves only the dragged card', async ({page}) => {
  const repo = `e2e-project-board-${randomString(8)}`;
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repo, autoInit: false})]);
  try {
    const projectID = await apiCreateProject(page.request, owner, repo, 'Board');
    await createProjectColumn(page.request, owner, repo, String(projectID), 'Backlog');
    await createProjectColumn(page.request, owner, repo, String(projectID), 'Done');
    for (const title of ['first', 'second', 'third']) {
      await apiCreateIssue(page.request, {owner, repo, title, projects: [projectID]});
    }

    const boardURL = `/${owner}/${repo}/projects/${projectID}`;
    await page.goto(boardURL);
    const columns = page.locator('.project-column');
    await expect(columns).toHaveCount(2);
    const [fromColumn, doneColumn] = [columns.first(), columns.last()];
    const cards = (column: typeof fromColumn) => column.locator('.issue-card');
    await expect(cards(fromColumn)).toHaveCount(3);
    const issueIDs = await cards(fromColumn).evaluateAll((els) => els.map((el) => el.getAttribute('data-issue')));
    const doneColumnID = await doneColumn.getAttribute('data-id');

    // another user moves the last card to "Done" while this board is stale
    const resp = await page.request.post(`${baseUrl()}${boardURL}/${doneColumnID}/move`, {
      headers: apiHeaders(),
      data: {issueID: Number(issueIDs[2]), prevIssueID: 0},
    });
    expect(resp.ok()).toBeTruthy();

    // drag "second" above "first" on the stale board
    const moved = page.waitForResponse((r) => r.url().endsWith('/move') && r.request().method() === 'POST');
    await cards(fromColumn).nth(1).dragTo(cards(fromColumn).first(), {targetPosition: {x: 10, y: 2}});
    expect((await moved).ok()).toBeTruthy();

    await page.goto(boardURL);
    await expect(cards(fromColumn)).toHaveCount(2);
    await expect(cards(fromColumn).first()).toContainText('second');
    await expect(cards(fromColumn).last()).toContainText('first');
    await expect(cards(doneColumn)).toHaveCount(1);
    await expect(cards(doneColumn).first()).toContainText('third');
  } finally {
    await apiDeleteRepo(page.request, owner, repo);
  }
});
