#!/usr/bin/env node
import {env, exit} from 'node:process';
import {labelsForPrTitle, managedLabels} from './pr-title.ts';

const title = env.PR_TITLE;

if (!title) {
  console.error('Missing PR_TITLE');
  exit(1);
}

const token = env.GITHUB_TOKEN;
const repository = env.GITHUB_REPOSITORY;
const prNumber = env.PR_NUMBER;

if (!token || !repository || !prNumber) {
  console.info('Skipping PR label sync (GITHUB_TOKEN, GITHUB_REPOSITORY, or PR_NUMBER not set)');
  exit(0);
}

const [owner, repo] = repository.split('/');
if (!owner || !repo) {
  console.error(`Invalid GITHUB_REPOSITORY: ${repository}`);
  exit(1);
}

const apiBase = `https://api.github.com/repos/${owner}/${repo}`;

async function githubRequest(path: string, options: {method?: string; body?: unknown} = {}): Promise<unknown> {
  const response = await fetch(`${apiBase}${path}`, {
    method: options.method ?? 'GET',
    headers: {
      Accept: 'application/vnd.github+json',
      Authorization: `Bearer ${token}`,
      'X-GitHub-Api-Version': '2022-11-28',
      ...(options.body ? {'Content-Type': 'application/json'} : {}),
    },
    body: options.body ? JSON.stringify(options.body) : undefined,
  });

  if (!response.ok) {
    const body = await response.text();
    throw new Error(`GitHub API ${options.method ?? 'GET'} ${path} failed (${response.status}): ${body}`);
  }

  if (response.status === 204) return null;
  return response.json();
}

async function getCurrentLabelNames(): Promise<string[]> {
  const labels = await githubRequest(`/issues/${prNumber}/labels`) as Array<{name: string}>;
  return labels.map((label) => label.name);
}

async function addLabels(names: string[]): Promise<void> {
  if (names.length === 0) return;
  await githubRequest(`/issues/${prNumber}/labels`, {method: 'POST', body: {labels: names}});
}

async function removeLabel(name: string): Promise<void> {
  await githubRequest(`/issues/${prNumber}/labels/${encodeURIComponent(name)}`, {method: 'DELETE'});
}

const desiredLabels = labelsForPrTitle(title);

try {
  const currentLabels = await getCurrentLabelNames();
  const labelsToRemove = managedLabels.filter((name) => currentLabels.includes(name) && !desiredLabels.includes(name));
  const labelsToAdd = desiredLabels.filter((name) => !currentLabels.includes(name));

  for (const name of labelsToRemove) {
    await removeLabel(name);
    console.info(`Removed label ${name}`);
  }

  if (labelsToAdd.length > 0) {
    await addLabels(labelsToAdd);
    console.info(`Added labels: ${labelsToAdd.join(', ')}`);
  }

  if (labelsToRemove.length === 0 && labelsToAdd.length === 0) {
    console.info('PR labels already in sync');
  }
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  exit(1);
}
