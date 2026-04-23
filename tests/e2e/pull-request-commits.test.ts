import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {
  login,
  randomString,
  apiCreateRepo,
  apiCreateBranch,
  apiCreateFile,
  apiCreatePullRequest,
  apiMergePullRequest,
  apiGetPullRequestCommits,
  apiDeleteRepo,
} from './utils.ts';

/**
 * Regression test for: PRs are showing already merged commits
 * https://github.com/go-gitea/gitea/issues/37383
 *
 * When a commit reaches a branch via two different merge paths, Gitea incorrectly
 * lists it as a new commit in a subsequent PR that targets that branch.
 *
 * Scenario:
 *   1. commit A is added to `feature`
 *   2. PR feature → staging is merged  (commit A is now in staging)
 *   3. PR feature → develop is merged  (commit A is now in develop)
 *   4. PR develop → staging is opened
 *   → BUG: PR #4 lists commit A as new, even though it is already in staging
 *   → FIX: PR #4 lists 0 commits (nothing new for staging)
 */
test('PR should not list commits already present in the target branch via another merge path', async ({page, request}) => {
  const owner = env.GITEA_TEST_E2E_USER;
  const repo = `e2e-pr-commits-${randomString(6)}`;

  await apiCreateRepo(request, {name: repo, autoInit: true});

  try {
    // Create branches from main
    await apiCreateBranch(request, owner, repo, 'staging');
    await apiCreateBranch(request, owner, repo, 'develop');
    await apiCreateBranch(request, owner, repo, 'feature');

    // Add a commit on feature (must specify branch — default would commit to main)
    await apiCreateFile(request, owner, repo, 'feature.txt',
      `feature content ${randomString(8)}`, 'feature');

    // PR 1: feature → staging, merge it
    const pr1 = await apiCreatePullRequest(request, owner, repo, {
      head: 'feature', base: 'staging', title: 'feature into staging',
    });
    await apiMergePullRequest(request, owner, repo, pr1);

    // PR 2: feature → develop, merge it
    const pr2 = await apiCreatePullRequest(request, owner, repo, {
      head: 'feature', base: 'develop', title: 'feature into develop',
    });
    await apiMergePullRequest(request, owner, repo, pr2);

    // PR 3: develop → staging (the one that should show 0 new commits)
    const pr3 = await apiCreatePullRequest(request, owner, repo, {
      head: 'develop', base: 'staging', title: 'develop into staging',
    });

    // Verify via UI that the PR commits page is reachable and shows the correct state
    await login(page);
    await page.goto(`/${owner}/${repo}/pulls/${pr3}/commits`);
    await page.waitForLoadState('networkidle');

    // Screenshot before assertion — proves we reached the correct PR commits page
    await page.screenshot({path: 'test-results/pr-commits-tab.png'});

    const commits = await apiGetPullRequestCommits(request, owner, repo, pr3);

    // The commit from `feature` is already in `staging` via PR 1.
    // A correct implementation lists 0 new commits for this PR.
    expect(commits, 'PR develop→staging must not list commits already present in staging').toHaveLength(0);
  } finally {
    await apiDeleteRepo(request, owner, repo);
  }
});
