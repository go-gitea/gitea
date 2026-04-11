import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {login, apiCreateRepo, apiCreateFile, apiDeleteRepo, assertNoJsError, randomString} from './utils.ts';

test('external file', async ({page, request}) => {
  const repoName = `e2e-external-render-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([
    apiCreateRepo(request, {name: repoName}),
    login(page),
  ]);
  try {
    await apiCreateFile(request, owner, repoName, 'test.external', '<p>rendered content</p>');
    await page.goto(`/${owner}/${repoName}/src/branch/main/test.external`);
    const iframe = page.locator('iframe.external-render-iframe');
    await expect(iframe).toBeVisible();
    await expect(iframe).toHaveAttribute('data-src', new RegExp(`/${owner}/${repoName}/render/branch/main/test\\.external`));
    const frame = page.frameLocator('iframe.external-render-iframe');
    await expect(frame.locator('p')).toContainText('rendered content');
    await assertNoJsError(page);
  } finally {
    await apiDeleteRepo(request, owner, repoName);
  }
});

test('openapi file', async ({page, request}) => {
  const repoName = `e2e-openapi-render-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([
    apiCreateRepo(request, {name: repoName}),
    login(page),
  ]);
  try {
    const spec = 'openapi: "3.0.0"\ninfo:\n  title: Test API\n  version: "1.0"\npaths: {}\n';
    await apiCreateFile(request, owner, repoName, 'openapi.yaml', spec);
    await page.goto(`/${owner}/${repoName}/src/branch/main/openapi.yaml`);
    const iframe = page.locator('iframe.external-render-iframe');
    await expect(iframe).toBeVisible();
    const frame = page.frameLocator('iframe.external-render-iframe');
    await expect(frame.locator('#swagger-ui .swagger-ui')).toBeVisible();
    await assertNoJsError(page);
  } finally {
    await apiDeleteRepo(request, owner, repoName);
  }
});
