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
 * Git topology after setup:
 *   main:    A
 *   feature: A → B          (B = the feature file commit)
 *   staging: A → M1         (M1 = merge commit "feature into staging"; M1 contains B)
 *   develop: A → M2         (M2 = merge commit "feature into develop"; M2 contains B)
 *
 * PR develop → staging (PR3):
 *   git log staging..develop = [M2]   (M2 is new in develop; B is already in staging via M1)
 *
 *   → BUG: PR3 lists [M2, B] (2 commits) — B incorrectly shown as new for staging
 *   → FIX: PR3 lists [M2]   (1 commit)  — only the genuinely new merge commit appears
 */
test('PR should not list commits already present in the target branch via another merge path', async ({page, request}) => {
  const owner = env.GITEA_TEST_E2E_USER;
  const repo = `e2e-pr-commits-${randomString(6)}`;

  await apiCreateRepo(request, {name: repo, autoInit: true});

  try {
    // Create branches and log in concurrently — all independent after repo exists
    await Promise.all([
      apiCreateBranch(request, owner, repo, 'staging'),
      apiCreateBranch(request, owner, repo, 'develop'),
      apiCreateBranch(request, owner, repo, 'feature'),
      login(page),
    ]);

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

    // PR 3: develop → staging (the one being tested)
    const pr3 = await apiCreatePullRequest(request, owner, repo, {
      head: 'develop', base: 'staging', title: 'develop into staging',
    });

    // Fetch commits via API and navigate to the PR page concurrently
    const [commits] = await Promise.all([
      apiGetPullRequestCommits(request, owner, repo, pr3),
      page.goto(`/${owner}/${repo}/pulls/${pr3}/commits`)
        .then(() => page.waitForLoadState('load'))
        .then(() => page.screenshot({path: 'test-results/pr-commits-tab.png'})),
    ]);

    // M2 (the merge commit "feature into develop") is genuinely new in develop and
    // should appear. The feature file commit B must NOT appear — it is already
    // reachable from staging via M1. A buggy Gitea lists both (length 2); a correct
    // implementation lists only M2 (length 1).
    expect(commits, 'PR develop→staging must not list commits already present in staging').toHaveLength(1);
    expect(commits[0].commit.message, 'Only the develop merge commit should appear — not the feature file commit').toContain('feature into develop');
  } finally {
    await apiDeleteRepo(request, owner, repo);
  }
});
