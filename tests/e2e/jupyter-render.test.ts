import {env} from 'node:process';
import {expect, test} from '@playwright/test';
import {login, apiCreateRepo, apiCreateFile, assertNoJsError, randomString} from './utils.ts';

test.describe('jupyter notebook rendering', () => {
  let repoName: string;
  let owner: string;

  test.beforeAll(async ({request}) => {
    repoName = `e2e-jupyter-${randomString(8)}`;
    owner = env.GITEA_TEST_E2E_USER;

    await apiCreateRepo(request, {name: repoName});

    // Single comprehensive test notebook
    const notebook = JSON.stringify({
      cells: [
        {cell_type: 'markdown', source: ['# Header 1\n', '## Header 2\n', '**bold** *italic* `code`\n', '- List item 1\n', '- List item 2\n', '[link](https://example.com)\n', '| Col1 | Col2 |\n', '|------|------|\n', '| A | B |\n', '```python\ncode block\n```\n', '> blockquote\n', '~~strikethrough~~']},
        {cell_type: 'code', execution_count: 1, source: ['print("Hello")'], outputs: [{output_type: 'stream', name: 'stdout', text: ['Hello\n']}]},
        {cell_type: 'code', execution_count: 2, source: ['x'], outputs: [{output_type: 'execute_result', data: {'image/png': 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=='}}]},
        {cell_type: 'code', source: ['# No output'], outputs: []},
        {cell_type: 'code', source: ['err'], outputs: [{output_type: 'error', ename: 'ValueError', evalue: 'Test', traceback: ['ValueError: Test']}]},
        {cell_type: 'code', source: ['mixed'], outputs: [{output_type: 'stream', name: 'stdout', text: ['text\n']}, {output_type: 'execute_result', data: {'text/html': ['<b>HTML</b>']}}]},
      ],
      metadata: {}, nbformat: 4, nbformat_minor: 5,
    });

    await apiCreateFile(request, owner, repoName, 'test.ipynb', notebook);
  });

  test('renders markdown cells', async ({page}) => {
    await login(page);
    await page.goto(`/${owner}/${repoName}/src/branch/main/test.ipynb`);
    await assertNoJsError(page);
    const frame = page.frameLocator('iframe.external-render-iframe');
    await expect(frame.locator('.cell.markdown h1')).toBeVisible();
    await expect(frame.locator('.cell.markdown strong')).toBeVisible();
    await expect(frame.locator('.cell.markdown ul li').first()).toBeVisible();
    await expect(frame.locator('.cell.markdown table')).toBeVisible();
  });

  test('renders code cells with outputs', async ({page}) => {
    await login(page);
    await page.goto(`/${owner}/${repoName}/src/branch/main/test.ipynb`);
    await assertNoJsError(page);
    await expect(page.frameLocator('iframe.external-render-iframe').locator('.cell.code .output pre').first()).toBeVisible();
  });

  test('renders image outputs', async ({page}) => {
    await login(page);
    await page.goto(`/${owner}/${repoName}/src/branch/main/test.ipynb`);
    await assertNoJsError(page);
    await expect(page.frameLocator('iframe.external-render-iframe').locator('.cell.code .output img')).toBeVisible();
  });

  test('renders error outputs', async ({page}) => {
    await login(page);
    await page.goto(`/${owner}/${repoName}/src/branch/main/test.ipynb`);
    await assertNoJsError(page);
    await expect(page.frameLocator('iframe.external-render-iframe').locator('.error-output')).toBeVisible();
  });
});
