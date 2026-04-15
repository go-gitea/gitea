import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {apiCreateRepo, apiCreateFile, apiDeleteRepo, assertNoJsError, randomString} from './utils.ts';

test('3d model file', async ({page, request}) => {
  const repoName = `e2e-3d-render-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await apiCreateRepo(request, {name: repoName});
  try {
    const stl = 'solid test\nfacet normal 0 0 1\nouter loop\nvertex 0 0 0\nvertex 1 0 0\nvertex 0 1 0\nendloop\nendfacet\nendsolid test\n';
    await apiCreateFile(request, owner, repoName, 'test.stl', stl);
    await page.goto(`/${owner}/${repoName}/src/branch/main/test.stl?display=rendered`);
    const iframe = page.locator('iframe.external-render-iframe');
    await expect(iframe).toBeVisible();
    await expect(page.frameLocator('iframe.external-render-iframe').locator('#viewer canvas')).toBeVisible();
    await assertNoJsError(page);
  } finally {
    await apiDeleteRepo(request, owner, repoName);
  }
});
