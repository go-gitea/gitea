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

function parsePrTitle(title: string): {type: CommitType; breaking: boolean} | null {
  const match = titlePattern.exec(title);
  return match ? {type: match[1] as CommitType, breaking: Boolean(match[3])} : null;
}

const breakingLabel = 'pr/breaking';

// Mutually exclusive type labels, fully synced with the title type (added and removed).
const typeLabels: Partial<Record<CommitType, string>> = {
  feat: 'type/feature',
  enhance: 'type/enhancement',
  fix: 'type/bug',
  docs: 'type/docs',
  test: 'type/testing',
};

// Non-type labels, only added, never auto-removed, so manual labeling is not clobbered.
const extraLabels: Partial<Record<CommitType, string>> = {
  chore: 'skip-changelog',
  ci: 'skip-changelog',
  build: 'topic/build',
};

// Labels this tool may remove when the title no longer implies them.
const removableLabels = [...Object.values(typeLabels), breakingLabel];

function labelsForPrTitle(title: string): string[] {
  const parsed = parsePrTitle(title);
  if (!parsed) return [];
  return [typeLabels[parsed.type], extraLabels[parsed.type], parsed.breaking ? breakingLabel : undefined]
    .filter((label): label is string => label !== undefined);
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

  async function request(url: string, method = 'GET', body?: unknown): Promise<Response> {
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
    if (!response.ok) {
      throw new Error(`GitHub API ${method} ${url} failed (${response.status}): ${await response.text()}`);
    }
    return response;
  }

  const desired = labelsForPrTitle(env.PR_TITLE);
  const response = await request(`${labelsUrl}?per_page=100`);
  const current = ((await response.json()) as Array<{name: string}>).map((label) => label.name);

  const toAdd = desired.filter((name) => !current.includes(name));
  const toRemove = removableLabels.filter((name) => current.includes(name) && !desired.includes(name));

  if (toAdd.length) {
    await request(labelsUrl, 'POST', {labels: toAdd});
    console.info(`Added labels: ${toAdd.join(', ')}`);
  }
  for (const name of toRemove) {
    await request(`${labelsUrl}/${encodeURIComponent(name)}`, 'DELETE');
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
