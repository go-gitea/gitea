import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {login, apiCreateRepo, apiCreateFile, assertFlushWithParent, assertNoJsError, randomString} from './utils.ts';

test('external file', async ({page, request}) => {
  const repoName = `e2e-external-render-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([
    apiCreateRepo(request, {name: repoName}),
    login(page),
  ]);
  await apiCreateFile(request, owner, repoName, 'test.external', '<p>rendered content</p>');
  await page.goto(`/${owner}/${repoName}/src/branch/main/test.external`);
  const iframe = page.locator('iframe.external-render-iframe');
  await expect(iframe).toBeVisible();
  await expect(iframe).toHaveAttribute('data-src', new RegExp(`/${owner}/${repoName}/render/branch/main/test\\.external`));
  const frame = page.frameLocator('iframe.external-render-iframe');
  await expect(frame.locator('p')).toContainText('rendered content');
  await assertFlushWithParent(iframe, page.locator('.file-view'));
  await assertNoJsError(page);
});

test('openapi file', async ({page, request}) => {
  const repoName = `e2e-openapi-render-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([
    apiCreateRepo(request, {name: repoName}),
    login(page),
  ]);
  const title = 'Test <API> & "quoted"';
  const spec = JSON.stringify({
    openapi: '3.0.0',
    info: {title, version: '1.0'},
    paths: {'/pets': {get: {responses: {'200': {description: 'OK', content: {'application/json': {schema: {$ref: '#/components/schemas/Pet'}}}}}}}},
    components: {schemas: {Pet: {type: 'object', properties: {children: {type: 'array', items: {$ref: '#/components/schemas/Pet'}}}}}},
  });
  await apiCreateFile(request, owner, repoName, 'openapi.json', spec);
  await page.goto(`/${owner}/${repoName}/src/branch/main/openapi.json`);
  const iframe = page.locator('iframe.external-render-iframe');
  await expect(iframe).toBeVisible();
  const viewer = page.frameLocator('iframe.external-render-iframe').locator('#frontend-render-viewer');
  await expect(viewer.locator('.swagger-ui')).toBeVisible();
  await expect(viewer.locator('.info .title')).toContainText(title);
  // expanding the operation triggers swagger-ui's $ref resolver, which fetches window.location
  // (about:srcdoc since the iframe is loaded via srcdoc); failure surfaces as "Could not resolve reference"
  await viewer.locator('.opblock-tag').first().click();
  await viewer.locator('.opblock').first().click();
  await expect(viewer.getByText('Could not resolve reference')).toHaveCount(0);
  // poll: postMessage resize may not have settled yet when the visibility checks pass
  await expect.poll(async () => (await iframe.boundingBox())!.height).toBeGreaterThan(300);
  await assertFlushWithParent(iframe, page.locator('.file-view'));
  await assertNoJsError(page);
});
