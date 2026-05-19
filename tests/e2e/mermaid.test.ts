import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {apiCreateRepo, apiHeaders, assertNoJsError, baseUrl, randomString} from './utils.ts';

test('mermaid diagram in issue', async ({page, request}) => {
  const repoName = `e2e-mermaid-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await apiCreateRepo(request, {name: repoName});
  const body = '```mermaid\nflowchart LR\n  Alpha --> Beta\n  Beta --> Gamma\n```\n';
  const response = await request.post(`${baseUrl()}/api/v1/repos/${owner}/${repoName}/issues`, {
    headers: apiHeaders(),
    data: {title: 'mermaid test', body},
  });
  expect(response.ok(), `create issue failed: ${response.status()}`).toBe(true);
  const {number} = await response.json();
  await page.goto(`/${owner}/${repoName}/issues/${number}`);

  const svg = page.frameLocator('iframe.markup-content-iframe').locator('svg');
  await expect(svg).toContainText(/Alpha[\s\S]*Beta[\s\S]*Gamma/);

  await assertNoJsError(page);
});
