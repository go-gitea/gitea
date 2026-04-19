import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {apiCreateRepo, apiCreateFile, apiDeleteRepo, assertFlushWithParent, assertNoJsError, login, randomString} from './utils.ts';

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
  } finally {
    await apiDeleteRepo(request, owner, repoName);
  }
});

test('pdf file', async ({page, request}) => {
  // headless playwright cannot render PDFs (PDFObject.embed returns false), so this is a limited test
  const repoName = `e2e-pdf-render-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await apiCreateRepo(request, {name: repoName});
  try {
    await apiCreateFile(request, owner, repoName, 'test.pdf', '%PDF-1.0\n%%EOF\n');
    await page.goto(`/${owner}/${repoName}/src/branch/main/test.pdf`);
    const container = page.locator('.file-view-render-container');
    await expect(container).toHaveAttribute('data-render-name', 'pdf-viewer');
    expect((await container.boundingBox())!.height).toBeGreaterThan(300);
    await assertFlushWithParent(container, page.locator('.file-view'));
  } finally {
    await apiDeleteRepo(request, owner, repoName);
  }
});

test('asciicast file', async ({page, request}) => {
  const repoName = `e2e-asciicast-render-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await Promise.all([apiCreateRepo(request, {name: repoName}), login(page)]);
  try {
    const cast = '{"version": 2, "width": 80, "height": 24}\n[0.0, "o", "test"]\n';
    await apiCreateFile(request, owner, repoName, 'test.cast', cast);
    await page.goto(`/${owner}/${repoName}/src/branch/main/test.cast`);
    const iframe = page.locator('iframe.external-render-iframe');
    await expect(iframe).toBeVisible();
    const frame = page.frameLocator('iframe.external-render-iframe');
    const wrapper = frame.locator('.ap-wrapper');
    await expect(wrapper).toBeVisible();
    await expect(wrapper).toContainText('test');
    await expect.poll(async () => (await iframe.boundingBox())!.height).toBeGreaterThan(300);
    await assertFlushWithParent(iframe, page.locator('.file-view'));
    await assertNoJsError(page);
  } finally {
    await apiDeleteRepo(request, owner, repoName);
  }
});
