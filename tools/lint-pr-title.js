#!/usr/bin/env node
import {env, exit} from 'node:process';

const allowedTypes = 'build, chore, ci, docs, feat, fix, perf, refactor, revert, style, test';
const title = env.PR_TITLE;

if (!title) {
  console.error('Missing PR_TITLE');
  exit(1);
}

const validTitlePattern = new RegExp(`^(${allowedTypes.replaceAll(', ', '|')})(\\([\\w.-]+\\))?(!)?: .+$`);

if (!validTitlePattern.test(title)) {
  console.error(`Invalid PR title: ${title}`);
  console.error('Expected format: type(scope): subject');
  console.error(`Allowed types: ${allowedTypes}`);
  exit(1);
}
