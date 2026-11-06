import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {apiCreateRepo, apiCreateIssue, assertNoJsError, randomString} from './utils.ts';

test('mermaid diagram in issue', async ({page, request}) => {
  const repoName = `e2e-mermaid-${randomString(8)}`;
  const owner = env.GITEA_TEST_E2E_USER;
  await apiCreateRepo(request, {name: repoName, autoInit: false});
  const source = 'flowchart TD\n  Alpha --> Beta --> Gamma\n  Alpha --> Node1 & Node2 & Node3 & Node4 & Node5 & Node6 & Node7 & Node8';
  const {index} = await apiCreateIssue(request, {owner, repo: repoName, title: 'mermaid test', body: `\`\`\`mermaid\n${source}\n\`\`\`\n`});
  await page.goto(`/${owner}/${repoName}/issues/${index}`);

  const svg = page.frameLocator('iframe.markup-content-iframe').locator('svg');
  await expect(svg).toContainText(/Alpha[\s\S]*Beta[\s\S]*Gamma/);
  const [iframeWidth, iframeHeight, svgWidth, svgHeight] = await page.locator('iframe.markup-content-iframe').evaluate((iframe: HTMLIFrameElement) => {
    const iframeRect = iframe.getBoundingClientRect();
    const svgRect = iframe.contentDocument!.querySelector('svg')!.getBoundingClientRect();
    return [iframeRect.width, iframeRect.height, svgRect.width, svgRect.height];
  });
  expect(svgWidth).toBeCloseTo(iframeWidth, 0);
  expect(svgHeight).toBeCloseTo(iframeHeight, 0);

  await svg.hover();
  await page.locator('.code-copy').click();
  await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(source);

  await assertNoJsError(page);
});
