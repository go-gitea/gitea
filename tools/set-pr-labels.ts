#!/usr/bin/env node
import {env, exit} from 'node:process';
import {labelsForPrTitle, removableLabels} from './pr-title.ts';

if (!env.PR_TITLE) {
  console.error('Missing PR_TITLE');
  exit(1);
}

if (!env.GITHUB_TOKEN || !env.GITHUB_REPOSITORY || !env.PR_NUMBER) {
  console.info('Skipping PR label sync (GITHUB_TOKEN, GITHUB_REPOSITORY, or PR_NUMBER not set)');
  exit(0);
}

const labelsUrl = `https://api.github.com/repos/${env.GITHUB_REPOSITORY}/issues/${env.PR_NUMBER}/labels`;

async function request(url: string, method = 'GET', body?: unknown): Promise<Response> {
  const response = await fetch(url, {
    method,
    headers: {
      Accept: 'application/vnd.github+json',
      Authorization: `Bearer ${env.GITHUB_TOKEN}`,
      'X-GitHub-Api-Version': '2022-11-28',
      ...(body ? {'Content-Type': 'application/json'} : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    throw new Error(`GitHub API ${method} ${url} failed (${response.status}): ${await response.text()}`);
  }
  return response;
}

try {
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
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  exit(1);
}
