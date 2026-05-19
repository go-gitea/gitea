#!/usr/bin/env node
import {env, exit} from 'node:process';
import {allowedTypesList, parsePrTitle} from './pr-title.ts';

const title = env.PR_TITLE;

if (!title) {
  console.error('Missing PR_TITLE');
  exit(1);
}

if (!parsePrTitle(title)) {
  console.error(`Invalid PR title: ${title}`);
  console.error('Expected format: type(scope): subject');
  console.error(`Allowed types: ${allowedTypesList}`);
  exit(1);
}
