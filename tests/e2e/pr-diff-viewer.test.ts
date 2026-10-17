import {test, expect, type APIRequestContext, type Page} from '@playwright/test';
import {apiChangeFiles, apiCreateFiles, apiCreatePR, apiCreateRepo, apiCreateReview, apiCreateUser, apiUserHeaders, baseUrl, loginUser, randomString} from './utils.ts';

let pageErrors: string[] = [];

test.beforeEach(({page}) => {
  pageErrors = [];
  page.on('pageerror', (error) => { pageErrors.push(error.message) });
});

test.afterEach(() => {
  expect(pageErrors).toEqual([]);
});

async function createUserRepo(request: APIRequestContext, prefix: string) {
  const user = `${prefix}-${randomString(8)}`;
  await apiCreateUser(request, user);
  const headers = apiUserHeaders(user);
  const repo = `e2e-${prefix}-${randomString(8)}`;
  await apiCreateRepo(request, {name: repo, headers});
  return {user, headers, repo};
}

async function expectFileRendered(page: Page, name: string, addedLines: number) {
  const box = page.locator(`.diff-file-box[data-new-filename="${name}"]`);
  await expect(box.locator('tr.add-code')).toHaveCount(addedLines);
  return box;
}

test('diff viewer', async ({page, request}) => {
  const {user, headers, repo} = await createUserRepo(request, 'dv');
  await apiCreateFiles(request, user, repo, [
    {path: 'src/a.ts', content: 'const a = 1;\nconst b = 2;\nconst c = 3;\n'},
    {path: 'src/b.ts', content: 'const b = 1;\n'},
  ], {branch: 'main', newBranch: 'feat', headers});
  const prIndex = await apiCreatePR(request, user, repo, 'feat', 'main', 'diff viewer', {headers});
  await apiCreateReview(request, user, repo, prIndex, {comments: [{path: 'src/a.ts', body: 'viewer-inline-comment', new_position: 1}], headers});
  const branch = await (await request.get(`${baseUrl()}/api/v1/repos/${user}/${repo}/branches/feat`, {headers})).json();
  const commitSha: string = branch.commit.id;
  await loginUser(page, user);

  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files`);
  const fileA = await expectFileRendered(page, 'src/a.ts', 3);
  // highlighted by the code editor's grammar
  await expect(fileA.locator('.tok-keyword').first()).toHaveText('const');

  // the existing comment renders like the legacy conversation
  const conversation = fileA.locator('.diff-conversation');
  await expect(conversation.locator('.comment-header-left')).toContainText(user);
  await expect(conversation.locator('.comment-body .markup')).toContainText('viewer-inline-comment');

  // the line "+" and the line numbers open independent composers
  await fileA.locator('tr[data-new="2"] .lines-code').hover();
  await fileA.locator('tr[data-new="2"] .add-code-comment').click();
  await fileA.locator('tr[data-new="3"] .lines-num-new').click();
  await expect(fileA.locator('.diff-viewer-draft')).toHaveCount(2);
  await expect(fileA.locator('.diff-viewer-draft markdown-toolbar[for]')).toHaveCount(2);

  // the file tree navigates
  await page.locator('#diff-file-tree').getByText('a.ts').click();
  await expect(page).toHaveURL(/#diff-[0-9a-f]+$/);

  // marking a file viewed folds it
  await Promise.all([page.waitForResponse(/\/viewed-files$/), fileA.getByLabel('Viewed').check()]);
  await expect(fileA).toHaveAttribute('data-folded', 'true');

  // the other diff pages use the same viewer
  await page.goto(`/${user}/${repo}/commit/${commitSha}`);
  await expectFileRendered(page, 'src/a.ts', 3);
  const wikiPage = await request.post(`${baseUrl()}/api/v1/repos/${user}/${repo}/wiki/new`, {headers, data: {title: 'Home', content_base64: btoa('wiki line\n')}});
  expect(wikiPage.ok()).toBe(true);
  const revisions = await (await request.get(`${baseUrl()}/api/v1/repos/${user}/${repo}/wiki/revisions/Home`, {headers})).json();
  await page.goto(`/${user}/${repo}/wiki/commit/${revisions.commits[0].sha}`);
  await expectFileRendered(page, 'Home.md', 1);
});

test('diff viewer review', async ({page, request}) => {
  const {user, headers, repo} = await createUserRepo(request, 'dvr');
  await apiCreateFiles(request, user, repo, [
    {path: 'a.txt', content: 'one\ntwo\nthree\nfour\n'},
  ], {branch: 'main', newBranch: 'feat', headers});
  const prIndex = await apiCreatePR(request, user, repo, 'feat', 'main', 'review', {headers});
  await apiCreateReview(request, user, repo, prIndex, {comments: [{path: 'a.txt', body: 'existing-comment', new_position: 1}], headers});
  await loginUser(page, user);
  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files`);

  const file = page.locator('.diff-file-box[data-new-filename="a.txt"]');
  const counter = page.locator('.review-box-button .review-comments-counter');
  const writeComment = async (line: string, text: string, button: string) => {
    await file.locator(`tr[data-new="${line}"] .lines-num-new`).click();
    const draft = file.locator('.diff-viewer-draft');
    await draft.locator('textarea.markdown-text-editor').fill(text);
    await draft.getByRole('button', {name: button}).click();
    await expect(file.locator('.diff-conversation').filter({hasText: text})).toBeVisible();
  };

  // the first comment starts a review, the next ones are added to it
  await writeComment('2', 'pending-one', 'Start review');
  await writeComment('3', 'pending-two', 'Add comment');
  await expect(counter).toHaveText('2');
  await expect(file.locator('.diff-conversation').filter({hasText: 'pending-two'}).locator('.pending-label')).toBeVisible();

  // conversation navigation follows the order of the lines
  const pendingTwoId = await file.locator('.comment').filter({hasText: 'pending-two'}).getAttribute('id');
  await file.locator('.diff-conversation').filter({hasText: 'pending-one'}).getByRole('button', {name: 'Next'}).click();
  await expect(page).toHaveURL(new RegExp(`#${pendingTwoId}$`));

  const existing = file.locator('.comment').filter({hasText: 'existing-comment'});
  const openMenuItem = async (comment: typeof existing, item: string) => {
    await comment.getByRole('menu').filter({has: page.getByRole('menuitem', {name: item, exact: true, includeHidden: true})}).click();
    await comment.getByRole('menuitem', {name: item, exact: true}).click();
  };

  // edit
  await openMenuItem(existing, 'Edit');
  const editor = existing.locator('.edit-content-zone textarea.markdown-text-editor');
  await expect(editor).toHaveValue('existing-comment'); // the editor is ready
  await editor.fill('edited-comment');
  await existing.getByRole('button', {name: 'Save'}).click();
  const edited = file.locator('.comment').filter({hasText: 'edited-comment'});
  await expect(edited.locator('.render-content')).toHaveText('edited-comment');

  // react
  await edited.locator('.select-reaction').first().click();
  await edited.locator('.select-reaction .item.emoji[data-reaction-content="+1"]').first().click();
  await expect(edited.locator('.bottom-reactions .reaction-count')).toHaveText('1');

  // quote reply
  await openMenuItem(edited, 'Quote reply');
  await expect(file.locator('.comment-code-cloud form textarea.markdown-text-editor').first()).toHaveValue(/^> edited-comment/);

  // resolve and deep link into the resolved conversation
  await file.getByRole('button', {name: 'Resolve conversation'}).click();
  await expect(file.locator('.resolved-placeholder')).toContainText('marked this conversation as resolved');
  const commentId = await edited.getAttribute('id');
  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files#${commentId}`);
  await expect(page.locator(`#${commentId}`)).toBeInViewport();
});

test('diff viewer options', async ({page, request}) => {
  const {user, headers, repo} = await createUserRepo(request, 'dvo');
  await apiCreateFiles(request, user, repo, [
    {path: 'code.ts', content: 'const foo = 1;\nconst same = 2;\n'},
    {path: 'space.txt', content: 'a b\n'},
  ], {branch: 'main', headers});
  await apiChangeFiles(request, user, repo, [{operation: 'update', path: 'code.ts', content: 'const bar = 1;\nconst same = 2;\n'}], {branch: 'main', newBranch: 'feat', headers});
  await apiChangeFiles(request, user, repo, [{operation: 'update', path: 'space.txt', content: 'a  b\n'}], {branch: 'feat', headers});
  const prIndex = await apiCreatePR(request, user, repo, 'feat', 'main', 'options', {headers});
  await loginUser(page, user);

  // unified view marks the changed words of paired lines
  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files?style=unified&whitespace=show-all`);
  const code = page.locator('.diff-file-box[data-new-filename="code.ts"]');
  await expect(code.locator('.removed-code')).toHaveText('foo');
  await expect(code.locator('.added-code')).toHaveText('bar');

  // the split view pairs the lines and comments go to their side
  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files?style=split&whitespace=show-all`);
  const pairedRow = code.locator('.code-diff-split tr.del-code');
  await expect(pairedRow.locator('.lines-num-old')).toHaveAttribute('data-line-num', '1');
  await expect(pairedRow.locator('.lines-num-new')).toHaveAttribute('data-line-num', '1');
  await pairedRow.locator('.lines-num-new').click();
  await expect(code.locator('.add-comment-right .diff-viewer-draft')).toHaveCount(1);

  // whitespace-only files are listed but have no changes after ignoring whitespace
  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files?style=unified&whitespace=ignore-all`);
  await expect(page.locator('.diff-file-box[data-new-filename="space.txt"]')).toContainText('Whitespace-only changes.');
  await expect(page.locator('.diff-file-box[data-new-filename="code.ts"] .removed-code')).toHaveText('foo');

  // the commit selector shows a single commit
  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files?style=unified&whitespace=show-all`);
  await page.locator('#diff-commit-select button').click();
  await page.locator('#diff-commit-select .item').filter({hasText: 'Update space.txt'}).click();
  await expect(page).toHaveURL(/\/commits\/[0-9a-f]+/);
  await expect(page.locator('.diff-file-box[data-new-filename="space.txt"] tr.add-code')).toHaveCount(1);
  await expect(page.locator('.diff-file-box[data-new-filename="code.ts"]')).toHaveCount(0);
});

test('diff viewer expand', async ({page, request}) => {
  const {user, headers, repo} = await createUserRepo(request, 'dve');
  const lines = Array.from({length: 60}, (_, i) => `line ${i + 1}`);
  await apiCreateFiles(request, user, repo, [{path: 'long.txt', content: `${lines.join('\n')}\n`}], {branch: 'main', headers});
  lines[29] = 'changed 30';
  await apiChangeFiles(request, user, repo, [{operation: 'update', path: 'long.txt', content: `${lines.join('\n')}\n`}], {branch: 'main', newBranch: 'feat', headers});
  const prIndex = await apiCreatePR(request, user, repo, 'feat', 'main', 'expand', {headers});
  await apiCreateReview(request, user, repo, prIndex, {comments: [{path: 'long.txt', body: 'hidden-comment', new_position: 50}], headers});
  await loginUser(page, user);
  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files`);

  const file = page.locator('.diff-file-box[data-new-filename="long.txt"]');
  await expect(file.locator('tr[data-line-type="same"]')).toHaveCount(6);
  // the comment is hidden in the collapsed lines below the hunk
  await expect(file.locator('.render-content').filter({hasText: 'hidden-comment'})).toHaveCount(0);
  const tail = file.locator('tr.tag-code').last();
  await expect(tail.locator('.code-comment-more')).toHaveText('1');

  // expanding above the hunk: 20 lines first, then the rest merges into the hunk
  await file.locator('tr.tag-code').first().locator('[data-expand="up"]').click();
  await expect(file.locator('tr[data-new="7"]')).toHaveCount(1);
  await expect(file.locator('tr.tag-code').first()).toContainText('@@ -7,27 +7,27 @@');
  await file.locator('tr.tag-code').first().locator('[data-expand="all"]').click();
  await expect(file.locator('tr[data-new="1"]')).toHaveCount(1);

  // expanding below the hunk shows the hidden comment
  await tail.locator('[data-expand="down"]').click();
  await expect(file.locator('.render-content').filter({hasText: 'hidden-comment'})).toBeVisible();
  await file.locator('tr.tag-code').last().locator('[data-expand="all"]').click();
  await expect(file.locator('tr[data-new="60"]')).toHaveCount(1);
  await expect(file.locator('.code-expander-button')).toHaveCount(0);

  // a link to a hidden comment expands its lines
  const commentId = await file.locator('.comment').filter({hasText: 'hidden-comment'}).getAttribute('id');
  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files#${commentId}`);
  await page.reload();
  await expect(page.locator(`#${commentId}`)).toBeInViewport();
});

test('diff viewer file kinds', async ({page, request}) => {
  const {user, headers, repo} = await createUserRepo(request, 'dvk');
  const changeFiles = (files: Parameters<typeof apiChangeFiles>[3], branch: string, newBranch?: string) => apiChangeFiles(request, user, repo, files, {branch, newBranch, headers});
  const redPixel = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg==';
  const bluePixel = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==';
  await changeFiles([{operation: 'create', path: '.gitattributes', content: '*.dat filter=lfs diff=lfs merge=lfs -text\n*.lfs.png filter=lfs diff=lfs merge=lfs -text\n'}], 'main');
  await changeFiles([
    {operation: 'create', path: 'pixel.png', base64: redPixel},
    {operation: 'create', path: 'table.csv', content: 'name,value\na,1\nb,2\n'},
    {operation: 'create', path: 'doc.dat', content: 'alpha\nbeta\n'},
    {operation: 'create', path: 'logo.lfs.png', base64: redPixel},
  ], 'main');
  await changeFiles([
    {operation: 'update', path: 'pixel.png', base64: bluePixel},
    {operation: 'update', path: 'table.csv', content: 'name,value\na,1\nb,3\n'},
    {operation: 'update', path: 'doc.dat', content: 'alpha\ngamma\n'},
    {operation: 'update', path: 'logo.lfs.png', base64: bluePixel},
    {operation: 'create', path: 'vendor/lib.js', content: 'var lib = 1;\n'},
    {operation: 'create', path: 'package-lock.json', content: '{}\n'},
  ], 'main', 'feat');
  const prIndex = await apiCreatePR(request, user, repo, 'feat', 'main', 'kinds', {headers});
  await loginUser(page, user);
  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files`);

  // images are compared by the image diff, the source view only says it's binary
  const image = page.locator('.diff-file-box[data-new-filename="pixel.png"]');
  await expect(image.locator('.image-diff .image-after')).toHaveCount(3);
  await expect(image.locator('.image-diff .bounds-info-after .bounds-info-width')).toHaveText('1px');
  await image.locator('.file-view-toggle').first().click();
  await expect(image.getByText('Binary file not shown.')).toBeVisible();

  // CSV files are compared by cells
  const table = page.locator('.diff-file-box[data-new-filename="table.csv"]');
  await expect(table.locator('.data-table td.modified')).toContainText('2');
  await expect(table.locator('.data-table td.modified .added-code')).toHaveText('3');

  // LFS objects are diffed by their content instead of the pointers
  const lfs = page.locator('.diff-file-box[data-new-filename="doc.dat"]');
  await expect(lfs.locator('.diff-file-header .ui.label').filter({hasText: 'LFS'})).toHaveCount(1);
  await expect(lfs.locator('tr.del-code .code-inner')).toHaveText('beta');
  await expect(lfs.locator('tr.add-code .code-inner')).toHaveText('gamma');
  const lfsImage = page.locator('.diff-file-box[data-new-filename="logo.lfs.png"]');
  await expect(lfsImage.locator('.image-diff')).toHaveAttribute('data-path-after', /\/media\/commit\/[0-9a-f]+\/logo\.lfs\.png$/);
  await expect(lfsImage.locator('.image-diff .bounds-info-before .bounds-info-width')).toHaveText('1px');

  // labels, generated files are folded
  await expect(page.locator('.diff-file-box[data-new-filename="vendor/lib.js"] .diff-file-header .ui.label')).toHaveText('Vendored');
  const generated = page.locator('.diff-file-box[data-new-filename="package-lock.json"]');
  await expect(generated.locator('.diff-file-header .ui.label')).toHaveText('Generated');
  await expect(generated).toHaveAttribute('data-folded', 'true');

  // the file menu
  await table.locator('[data-global-init="initDiffHeaderPopupMenu"]').click();
  await expect(page.getByRole('link', {name: 'View File'})).toHaveAttribute('href', /\/src\/commit\/[0-9a-f]+\/table\.csv$/);
  await expect(page.getByRole('button', {name: 'Copy filename'})).toBeVisible();
});

test('diff viewer highlighting and escaping', async ({page, request}) => {
  const {user, headers, repo} = await createUserRepo(request, 'dvh');
  await apiCreateFiles(request, user, repo, [
    {path: 'main.zig', content: 'pub fn main() void {\n    return;\n}\n'},
    {path: 'hidden.txt', content: 'admin\u{200B} = true\n'},
  ], {branch: 'main', newBranch: 'feat', headers});
  const prIndex = await apiCreatePR(request, user, repo, 'feat', 'main', 'highlight', {headers});
  await loginUser(page, user);
  await page.goto(`/${user}/${repo}/pulls/${prIndex}/files`);

  // zig has no CodeMirror grammar, it's highlighted by chroma's lexer
  const zig = page.locator('.diff-file-box[data-new-filename="main.zig"]');
  await expect(zig.locator('.chroma .kr, .chroma .k').first()).toHaveText(/^(pub|fn|return)$/);

  // the zero width space is escaped, and can be shown as is
  const hidden = page.locator('.diff-file-box[data-new-filename="hidden.txt"]');
  await expect(hidden.locator('.unicode-escaped .escaped-code-point')).toHaveAttribute('data-escaped', '[U+200B]');
  await hidden.locator('.toggle-escape-button').click();
  await expect(hidden.locator('.file-code')).not.toHaveClass(/unicode-escaped/);
});
