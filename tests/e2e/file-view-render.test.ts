import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {apiCreateRepo, apiCreateFile, assertFlushWithParent, assertNoJsError, login, randomString} from './utils.ts';

test('3d model file', async ({page, request}) => {
  const repoName = `e2e-3d-render-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await apiCreateRepo(request, {name: repoName});
  const stl = 'solid test\nfacet normal 0 0 1\nouter loop\nvertex 0 0 0\nvertex 1 0 0\nvertex 0 1 0\nendloop\nendfacet\nendsolid test\n';
  await apiCreateFile(request, owner, repoName, 'test.stl', stl);
  await page.goto(`/${owner}/${repoName}/src/branch/main/test.stl?display=rendered`);
  const iframe = page.locator('iframe.external-render-iframe');
  await expect(iframe).toBeVisible();
  const frame = page.frameLocator('iframe.external-render-iframe');
  const viewer = frame.locator('#frontend-render-viewer');
  await expect(viewer.locator('canvas')).toBeVisible();
  expect((await viewer.boundingBox())!.height).toBeGreaterThan(300);
  await assertFlushWithParent(iframe, page.locator('.file-view'));
  // bgcolor passed via gitea-iframe-bgcolor; 3D viewer reads it from body bgcolor — must match parent
  const [parentBg, iframeBg] = await Promise.all([
    page.evaluate(() => getComputedStyle(document.body).backgroundColor),
    frame.locator('body').evaluate((el) => getComputedStyle(el).backgroundColor),
  ]);
  expect(iframeBg).toBe(parentBg);
  await assertNoJsError(page);
});

test('pdf file', async ({page, request}) => {
  // headless playwright cannot render PDFs (PDFObject.embed returns false), so this is a limited test
  const repoName = `e2e-pdf-render-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await apiCreateRepo(request, {name: repoName});
  await apiCreateFile(request, owner, repoName, 'test.pdf', '%PDF-1.0\n%%EOF\n');
  await page.goto(`/${owner}/${repoName}/src/branch/main/test.pdf`);
  const container = page.locator('.file-view-render-container');
  await expect(container).toHaveAttribute('data-render-name', 'pdf-viewer');
  expect((await container.boundingBox())!.height).toBeGreaterThan(300);
  await assertFlushWithParent(container, page.locator('.file-view'));
});

test('asciicast file', async ({page, request}) => {
  // regression for repo_file.go's RefTypeNameSubURL double-escape: readme.cast on a non-ASCII branch
  // is rendered via view_readme.go (no metas override), exposing the bug as a broken player URL
  const repoName = `e2e-asciicast-render-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  const branch = '日本語-branch';
  const branchEnc = encodeURIComponent(branch);
  await Promise.all([apiCreateRepo(request, {name: repoName, autoInit: false}), login(page)]);
  const cast = '{"version": 2, "width": 80, "height": 24}\n[0.0, "o", "hi"]\n';
  // on an empty repo, apiCreateFile with newBranch creates that branch as the initial commit
  await apiCreateFile(request, owner, repoName, 'readme.cast', cast, {newBranch: branch});
  await page.goto(`/${owner}/${repoName}/src/branch/${branchEnc}`);
  const container = page.locator('.asciinema-player-container');
  await expect(container).toHaveAttribute('data-asciinema-player-src', `/${owner}/${repoName}/raw/branch/${branchEnc}/readme.cast`);
  await expect(container.locator('.ap-wrapper')).toBeVisible();
  expect((await container.boundingBox())!.height).toBeGreaterThan(300);
});
