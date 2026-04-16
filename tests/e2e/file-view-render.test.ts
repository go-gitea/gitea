import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {apiCreateBranch, apiCreateRepo, apiCreateFile, apiDeleteRepo, assertNoJsError, login, randomString} from './utils.ts';

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
    await expect(page.frameLocator('iframe.external-render-iframe').locator('#frontend-render-viewer canvas')).toBeVisible();
    await assertNoJsError(page);
  } finally {
    await apiDeleteRepo(request, owner, repoName);
  }
});

test('asciicast readme on non-ASCII branch', async ({page, request}) => {
  // regression for repo_file.go's RefTypeNameSubURL double-escape: readme.cast on a non-ASCII branch
  // is rendered via view_readme.go (no metas override), exposing the bug as a broken player URL
  const repoName = `e2e-asciicast-render-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  const branch = '日本語-branch';
  const branchEnc = encodeURIComponent(branch);
  await Promise.all([apiCreateRepo(request, {name: repoName, autoInit: false}), login(page)]);
  try {
    const cast = '{"version": 2, "width": 80, "height": 24}\n[0.0, "o", "hi"]\n';
    await apiCreateFile(request, owner, repoName, 'readme.cast', cast);
    await apiCreateBranch(request, owner, repoName, branch);
    await page.goto(`/${owner}/${repoName}/src/branch/${branchEnc}`);
    await expect(page.locator('.asciinema-player-container')).toHaveAttribute('data-asciinema-player-src', `/${owner}/${repoName}/raw/branch/${branchEnc}/readme.cast`);
    await expect(page.locator('.asciinema-player-container .ap-wrapper')).toBeVisible();
  } finally {
    await apiDeleteRepo(request, owner, repoName);
  }
});
