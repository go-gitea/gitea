import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {apiCreateRepo, apiDeleteRepo, apiHeaders, login, randomString} from './utils.ts';

test('label swimlanes keep card copies synchronized when changing status', async ({page}) => {
  const owner = env.GITEA_TEST_E2E_USER;
  const repo = `swimlanes-${randomString(8)}`;
  const api = `/api/v1/repos/${owner}/${repo}`;
  const headers = apiHeaders();
  await apiCreateRepo(page.request, {name: repo, autoInit: false});
  try {
    const post = async (path: string, data: Record<string, unknown>) => {
      const response = await page.request.post(`${api}${path}`, {headers, data});
      expect(response.ok()).toBeTruthy();
      return response.json();
    };
    const labelA = await post('/labels', {name: 'Label A', color: '#1565c0'});
    const labelB = await post('/labels', {name: 'Label B', color: '#00875a'});
    const project = await post('/projects', {title: 'Project A', template_type: 'none'});
    const todo = await post(`/projects/${project.id}/columns`, {title: 'TODO'});
    const done = await post(`/projects/${project.id}/columns`, {title: 'DONE'});
    const shared = await post('/issues', {title: 'Task A', labels: [labelA.id, labelB.id]});
    const unlabeled = await post('/issues', {title: 'Task B'});
    for (const issue of [shared, unlabeled]) {
      const response = await page.request.post(`${api}/projects/${project.id}/columns/${todo.id}/issues/${issue.id}`, {headers});
      expect(response.ok()).toBeTruthy();
    }
    await login(page);
    const board = `/${owner}/${repo}/projects/${project.id}`;
    await page.goto(board);
    await page.locator('.project-group-by').click();
    await page.locator('.project-group-by .menu').getByRole('menuitem', {name: 'Labels', exact: true}).click();
    await expect(page).toHaveURL(/group_by=labels/);
    await expect(page.locator('.project-swimlane')).toHaveCount(3);
    await expect(page.getByRole('link', {name: 'Task A', exact: true})).toHaveCount(2);
    const lane = page.locator(`.project-swimlane[data-label-id="${labelA.id}"]`);
    const card = lane.locator(`[data-issue="${shared.id}"]`);
    const destination = lane.locator(`.cards[data-board="${done.id}"]`);
    await card.dragTo(destination);
    await expect(lane.locator(`.cards[data-board="${done.id}"] [data-issue="${shared.id}"]`)).toBeVisible();
    await expect(page.locator(`.project-swimlane[data-label-id="${labelB.id}"] .cards[data-board="${done.id}"] [data-issue="${shared.id}"]`)).toBeVisible();
    const issueResponse = await page.request.get(`${api}/issues/${shared.number}`, {headers});
    const updated = await issueResponse.json();
    expect(updated.state).toBe('open');
    expect(updated.labels.map((label: {id: number}) => label.id).sort()).toEqual([labelA.id, labelB.id].sort());
    await expect(page.locator(`.project-swimlane[data-label-id="0"] .cards[data-board="${todo.id}"] [data-issue="${unlabeled.id}"]`)).toBeVisible();

    const moveURL = `**${board}/${todo.id}/move`;
    await page.route(moveURL, (route) => route.fulfill({status: 500, contentType: 'application/json', body: '{"errorMessage":"Move failed"}'}));
    const rejectedMove = page.waitForResponse((response) => response.url().endsWith(`${board}/${todo.id}/move`) && response.status() === 500);
    await card.dragTo(lane.locator(`.cards[data-board="${todo.id}"]`));
    await rejectedMove;
    await expect(lane.locator(`.cards[data-board="${done.id}"] [data-issue="${shared.id}"]`)).toBeVisible();
    await expect(page.locator('#project-board')).toHaveJSProperty('inert', false);
    await page.unroute(moveURL);

    await page.goto(`${board}?group_by=labels&labels=${labelA.id}`);
    await expect(page.locator('.project-swimlane')).toHaveCount(1);
    await expect(page.getByRole('link', {name: 'Task A', exact: true})).toHaveCount(1);
    await page.locator('.label-filter').click();
    await page.locator(`.label-filter .label-filter-query-item[data-label-id="${labelB.id}"]`).click();
    await expect(page.locator('.project-swimlane')).toHaveCount(2);
    await expect(page.getByRole('link', {name: 'Task A', exact: true})).toHaveCount(2);
    await page.locator('.project-group-by').click();
    await page.locator('.project-group-by .menu').getByRole('menuitem', {name: 'None', exact: true}).click();
    await expect(page.locator('.project-swimlane')).toHaveCount(0);
    await expect(page.getByRole('link', {name: 'Task A', exact: true})).toHaveCount(1);
  } finally {
    await apiDeleteRepo(page.request, owner, repo);
  }
});
