#!/usr/bin/env node
import {argv, env, exit} from 'node:process';

const allowedTypes = [
  'build',
  'chore',
  'ci',
  'docs',
  'enhance',
  'feat',
  'fix',
  'perf',
  'refactor',
  'revert',
  'style',
  'test',
] as const;
type CommitType = typeof allowedTypes[number];

const allowedTypesList = allowedTypes.join(', ');
const titlePattern = new RegExp(`^(${allowedTypes.join('|')})(\\([\\w/.-]+\\))?(!)?: .+$`);

function parsePrTitle(title: string): {type: CommitType, scope: string, breaking: boolean} | null {
  const match = titlePattern.exec(title);
  if (!match) return null;
  // Keep the first segment, so "webhook/discord" matches "webhook".
  const scope = match[2] ? match[2].slice(1, -1).toLowerCase().split('/')[0] : '';
  return {type: match[1] as CommitType, scope, breaking: Boolean(match[3])};
}

const breakingLabel = 'pr/breaking';

// Mutually exclusive type labels, fully synced with the title type (added and removed).
const typeLabels: Partial<Record<CommitType, string>> = {
  feat: 'type/feature',
  enhance: 'type/enhancement',
  fix: 'type/bug',
  docs: 'type/docs',
  test: 'type/testing',
  refactor: 'type/refactoring',
};

// Non-type labels, removed only when the previous title implied them.
const extraLabels: Partial<Record<CommitType, string>> = {
  chore: 'skip-changelog',
  ci: 'skip-changelog',
  build: 'topic/build',
};

// Scopes whose label differs from "topic/<scope>".
const scopeAliases: Record<string, string> = {
  actions: 'topic/gitea-actions',
  auth: 'topic/authentication',
  issue: 'topic/issues',
  markup: 'topic/content-rendering',
  oauth: 'topic/authentication',
  oauth2: 'topic/authentication',
  pull: 'topic/pr',
  pulls: 'topic/pr',
  webhook: 'topic/webhooks',
};

function labelForScope(scope: string): string | undefined {
  if (!scope) return undefined;
  return scopeAliases[scope] ?? `topic/${scope}`;
}

// Labels this tool may remove when the title no longer implies them.
const removableLabels = [...Object.values(typeLabels), breakingLabel];

function labelsForPrTitle(title: string): string[] {
  const parsed = parsePrTitle(title);
  if (!parsed) return [];
  const labels = [
    typeLabels[parsed.type],
    extraLabels[parsed.type],
    labelForScope(parsed.scope),
    parsed.breaking ? breakingLabel : undefined,
  ].filter((label): label is string => label !== undefined);
  return [...new Set(labels)];
}

// Command: validate PR_TITLE against the allowed Conventional Commits format.
function lintPrTitle(): void {
  if (!env.PR_TITLE) {
    console.error('Missing PR_TITLE');
    exit(1);
  }
  if (!parsePrTitle(env.PR_TITLE)) {
    console.error(`Invalid PR title: ${env.PR_TITLE}`);
    console.error('Expected format: type(scope): subject (scope optional, append "!" for breaking changes)');
    console.error(`Allowed types: ${allowedTypesList}`);
    exit(1);
  }
}

// Command: sync the title-derived labels onto the PR via the GitHub API.
async function setPrLabels(): Promise<void> {
  if (!env.PR_TITLE || !env.GITHUB_TOKEN || !env.GITHUB_REPOSITORY || !env.PR_NUMBER) {
    console.error('set-pr-labels requires PR_TITLE, GITHUB_TOKEN, GITHUB_REPOSITORY and PR_NUMBER');
    exit(1);
  }

  const labelsUrl = `https://api.github.com/repos/${env.GITHUB_REPOSITORY}/issues/${env.PR_NUMBER}/labels`;

  async function request(url: string, method = 'GET', body?: unknown, ignoreStatus?: number): Promise<Response> {
    const response = await fetch(url, {
      method,
      headers: {
        Accept: 'application/vnd.github+json',
        Authorization: `Bearer ${env.GITHUB_TOKEN}`,
        'X-GitHub-Api-Version': '2022-11-28',
        ...(Boolean(body) && {'Content-Type': 'application/json'}),
      },
      body: body ? JSON.stringify(body) : undefined,
    });
    if (!response.ok && response.status !== ignoreStatus) {
      throw new Error(`GitHub API ${method} ${url} failed (${response.status}): ${await response.text()}`);
    }
    return response;
  }

  async function fetchNames(url: string): Promise<string[]> {
    const names = [];
    for (let page = 1; page <= 10; page++) {
      const labels = (await (await request(`${url}?per_page=100&page=${page}`)).json()) as Array<{name: string}>;
      names.push(...labels.map((label) => label.name));
      if (labels.length < 100) break;
    }
    return names;
  }

  // Labels the repo does not have are dropped instead of failing the job.
  const candidates = labelsForPrTitle(env.PR_TITLE);
  const repoLabelsUrl = `https://api.github.com/repos/${env.GITHUB_REPOSITORY}/labels`;
  const [current, existing]: string[][] = await Promise.all([
    fetchNames(labelsUrl),
    candidates.length ? fetchNames(repoLabelsUrl) : [],
  ]);
  const desired = candidates.filter((name) => {
    if (existing.includes(name)) return true;
    console.info(`Skipping label not present on the repo: ${name}`);
    return false;
  });

  const stale = new Set([...removableLabels, ...labelsForPrTitle(env.PR_TITLE_BEFORE ?? '')]);
  const toAdd = desired.filter((name) => !current.includes(name));
  const toRemove = current.filter((name) => stale.has(name) && !desired.includes(name));

  if (toAdd.length) {
    await request(labelsUrl, 'POST', {labels: toAdd});
    console.info(`Added labels: ${toAdd.join(', ')}`);
  }
  for (const name of toRemove) {
    // 404 means the label is already gone.
    await request(`${labelsUrl}/${encodeURIComponent(name)}`, 'DELETE', undefined, 404);
    console.info(`Removed label: ${name}`);
  }
  if (!toAdd.length && !toRemove.length) {
    console.info('PR labels already in sync');
  }
}

const commands: Record<string, () => void | Promise<void>> = {
  'lint-pr-title': lintPrTitle,
  'set-pr-labels': setPrLabels,
};

const command = argv[2];
const handler = commands[command];
if (!handler) {
  console.error(`Usage: ci-tools.ts <${Object.keys(commands).join('|')}>`);
  exit(1);
}

try {
  await handler();
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  exit(1);
}
