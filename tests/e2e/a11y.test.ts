import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import type {APIRequestContext, Page} from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import {login, apiCreateRepo, apiCreateFiles, apiCreateIssue, apiCreatePR, randomString} from './utils.ts';

// Automated WCAG 2.1 A/AA checks. This only catches what axe can detect, manual testing is still needed.
const wcagTags = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'];
// TODO: dropdown triggers use role=menu while the menu items are hidden, they need a button+menu structure to satisfy this rule
const disabledRules = ['aria-required-children'];

async function scan(page: Page, urls: string[]) {
  const found: string[] = [];
  for (const url of urls) {
    for (const violation of await violationsOf(page, url)) found.push(`${url}: ${violation}`);
  }
  return found;
}

async function violationsOf(page: Page, url: string) {
  await page.goto(url);
  await page.locator('footer').waitFor();
  const {violations} = await new AxeBuilder({page})
    .withTags(wcagTags)
    .exclude('.disabled') // inactive components are exempt from contrast requirements
    .disableRules(disabledRules)
    .analyze();
  return violations.map((v) => `${v.id} (${v.impact}): ${v.help}\n${v.nodes.slice(0, 3).map((n) => `  ${n.target.join(' ')}\n  ${n.html.slice(0, 160)}`).join('\n')}`);
}

test('anonymous pages have no a11y violations', async ({page}) => {
  const urls = ['/user/login', '/user/sign_up', '/user/forgot_password', '/explore/repos', '/explore/users', '/explore/organizations'];
  expect(await scan(page, urls)).toEqual([]);
});

async function createRepo(request: APIRequestContext, page: Page, {withPR = false} = {}) {
  const owner = env.GITEA_TEST_E2E_USER;
  const repoName = `e2e-a11y-${randomString(8)}`;
  await apiCreateRepo(request, {name: repoName});
  await Promise.all([
    withPR && apiCreateFiles(request, owner, repoName, [{path: 'feat.txt', content: 'feature content\n'}], {branch: 'main', newBranch: 'feat'}),
    withPR && apiCreateIssue(request, {owner, repo: repoName, title: 'a11y issue', body: 'issue body'}),
    login(page),
  ]);
  return `/${owner}/${repoName}`;
}

test('user pages have no a11y violations', async ({page}) => {
  await login(page);
  expect(await scan(page, ['/', '/repo/create', '/user/settings', '/user/settings/applications', '/-/admin'])).toEqual([]);
});

test('repo pages have no a11y violations', async ({page, request}) => {
  const repo = await createRepo(request, page);
  expect(await scan(page, [repo, `${repo}/src/branch/main/README.md`, `${repo}/commits/branch/main`, `${repo}/branches`, `${repo}/settings`])).toEqual([]);
});

test('issue pages have no a11y violations', async ({page, request}) => {
  const repo = await createRepo(request, page, {withPR: true});
  expect(await scan(page, [`${repo}/issues`, `${repo}/issues/1`, `${repo}/issues/new`])).toEqual([]);
});

test('pull request pages have no a11y violations', async ({page, request}) => {
  const repo = await createRepo(request, page, {withPR: true});
  const [, owner, repoName] = repo.split('/');
  const index = await apiCreatePR(request, owner, repoName, 'feat', 'main', 'a11y pr');
  expect(await scan(page, [`${repo}/pulls/${index}`, `${repo}/pulls/${index}/files`])).toEqual([]);
});
