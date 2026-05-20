import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {
  login,
  randomString,
  apiCreateRepo,
  apiCreateBranch,
  apiCreateFile,
  apiCreatePR,
  apiMergePullRequest,
  apiGetPullRequestCommits,
  apiDeleteRepo,
} from './utils.ts';

/**
 * Regression test for: PRs are showing already merged commits
 * https://github.com/go-gitea/gitea/issues/37383
 *
 * When the same commit reaches a branch via two different merge paths, Gitea
 * incorrectly lists it as a new commit in a subsequent PR targeting that branch.
 *
 * Git topology after setup:
 *   main:    A
 *   feature: A → B          (B = the feature file commit)
 *   staging: A → M1         (M1 = merge commit "feature into staging"; M1 contains B)
 *   develop: A → M2         (M2 = merge commit "feature into develop"; M2 contains B)
 *
 * PR develop → staging (PR3):
 *   git log staging..develop = [M2]     correct: M2 is new; B is already in staging via M1
 *   git log staging..develop = [M2, B]  buggy: B incorrectly listed as new for staging
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

    // Add a commit on the feature branch (commit B in the diagram above)
    await apiCreateFile(request, owner, repo, 'feature.txt',
      `feature content ${randomString(8)}`, {branch: 'feature'});

    // PR1: feature → staging — merge creates M1
    const pr1 = await apiCreatePR(request, owner, repo, 'feature', 'staging', 'feature into staging');
    await apiMergePullRequest(request, owner, repo, pr1);

    // PR2: feature → develop — merge creates M2
    const pr2 = await apiCreatePR(request, owner, repo, 'feature', 'develop', 'feature into develop');
    await apiMergePullRequest(request, owner, repo, pr2);

    // PR3: develop → staging — the one being tested
    const pr3 = await apiCreatePR(request, owner, repo, 'develop', 'staging', 'develop into staging');

    // Fetch commits via API and navigate to the PR page concurrently
    const [commits] = await Promise.all([
      apiGetPullRequestCommits(request, owner, repo, pr3),
      page.goto(`/${owner}/${repo}/pulls/${pr3}/commits`)
        .then(() => page.waitForLoadState('load'))
        .then(() => page.screenshot({path: 'test-results/pr-commits-tab.png'})),
    ]);

    // M2 is genuinely new in develop and must appear.
    // B must NOT appear — it is already reachable from staging via M1.
    // Buggy Gitea returns [M2, B] (len 2); fixed Gitea returns [M2] (len 1).
    expect(commits, 'PR develop→staging must not list commits already present in staging').toHaveLength(1);
    expect(commits[0].commit.message, 'Only the develop merge commit should appear — not the feature file commit').toContain('feature into develop');
  } finally {
    await apiDeleteRepo(request, owner, repo);
  }
});
