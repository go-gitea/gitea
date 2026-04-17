#!/usr/bin/env node
import {globSync, readFileSync} from 'node:fs';
import {join, relative} from 'node:path';
import {fileURLToPath} from 'node:url';

type OccurrenceSource = 'class' | 'classList' | 'jquery' | 'css';

type Occurrence = {
  file: string,
  line: number,
  source: OccurrenceSource,
  snippet: string,
};

const repoRoot = fileURLToPath(new URL('..', import.meta.url));

// These are the currently known bare class names that collide with Tailwind utilities
// and are also used by Fomantic or repo-specific semantic classes.
const collisionTokens = new Set([
  'container',
  'filter',
  'fixed',
  'grid',
  'hidden',
  'inline',
  'table',
]);

const scanGlobs = [
  'templates/**/*.tmpl',
  'web_src/**/*.{css,js,ts,vue}',
  'modules/**/*.go',
  'models/**/*.go',
  'routers/**/*.go',
  'services/**/*.go',
];

const occurrences = new Map<string, Occurrence[]>();

function addOccurrence(token: string, occurrence: Occurrence) {
  if (!collisionTokens.has(token)) return;
  const list = occurrences.get(token) ?? [];
  list.push(occurrence);
  occurrences.set(token, list);
}

function getLineNumber(content: string, index: number) {
  return content.slice(0, index).split('\n').length;
}

function getLineSnippet(content: string, index: number) {
  const line = content.split('\n')[getLineNumber(content, index) - 1] ?? '';
  return line.trim();
}

function findClassAttrTokens(content: string, file: string) {
  for (const match of content.matchAll(/\bclass\s*=\s*(["'`])([\s\S]*?)\1/g)) {
    const classValue = match[2];
    const seen = new Set<string>();
    for (const token of classValue.split(/\s+/)) {
      if (!collisionTokens.has(token) || seen.has(token)) continue;
      seen.add(token);
      addOccurrence(token, {
        file,
        line: getLineNumber(content, match.index ?? 0),
        source: 'class',
        snippet: getLineSnippet(content, match.index ?? 0),
      });
    }
  }
}

function findClassListTokens(content: string, file: string) {
  for (const match of content.matchAll(/\bclassList\.(?:add|remove|toggle|contains)\(([^)]+)\)/g)) {
    const args = match[1];
    const seen = new Set<string>();
    for (const tokenMatch of args.matchAll(/["'`]([A-Za-z0-9_-]+)["'`]/g)) {
      const token = tokenMatch[1];
      if (!collisionTokens.has(token) || seen.has(token)) continue;
      seen.add(token);
      addOccurrence(token, {
        file,
        line: getLineNumber(content, match.index ?? 0),
        source: 'classList',
        snippet: getLineSnippet(content, match.index ?? 0),
      });
    }
  }
}

function findJQueryTokens(content: string, file: string) {
  for (const match of content.matchAll(/\b(?:addClass|removeClass|hasClass)\(([^)]+)\)/g)) {
    const args = match[1];
    const seen = new Set<string>();
    for (const tokenMatch of args.matchAll(/["'`]([A-Za-z0-9_-]+)["'`]/g)) {
      const token = tokenMatch[1];
      if (!collisionTokens.has(token) || seen.has(token)) continue;
      seen.add(token);
      addOccurrence(token, {
        file,
        line: getLineNumber(content, match.index ?? 0),
        source: 'jquery',
        snippet: getLineSnippet(content, match.index ?? 0),
      });
    }
  }
}

function findCssSelectorTokens(content: string, file: string) {
  for (const token of collisionTokens) {
    const pattern = new RegExp(`(^|[^\\\\w-])\\\\.${token}(?![\\\\w-])`, 'g');
    for (const match of content.matchAll(pattern)) {
      addOccurrence(token, {
        file,
        line: getLineNumber(content, match.index ?? 0),
        source: 'css',
        snippet: getLineSnippet(content, match.index ?? 0),
      });
    }
  }
}

for (const pattern of scanGlobs) {
  for (const relativePath of globSync(pattern, {cwd: repoRoot})) {
    const file = join(repoRoot, relativePath);
    const content = readFileSync(file, 'utf8');
    const relativeFile = relative(repoRoot, file);
    findClassAttrTokens(content, relativeFile);
    findClassListTokens(content, relativeFile);
    findJQueryTokens(content, relativeFile);
    if (file.endsWith('.css')) findCssSelectorTokens(content, relativeFile);
  }
}

const tokens = Array.from(occurrences.keys()).sort();
if (!tokens.length) {
  console.info('No configured Tailwind collision tokens found.');
  process.exit(0);
}

console.info('Tailwind collision audit');
console.info('');
for (const token of tokens) {
  const list = occurrences.get(token) ?? [];
  console.info(`${token}: ${list.length} occurrence(s)`);
  for (const occurrence of list.slice(0, 20)) {
    console.info(`  - [${occurrence.source}] ${occurrence.file}:${occurrence.line} ${occurrence.snippet}`);
  }
  if (list.length > 20) {
    console.info(`  - ... ${list.length - 20} more`);
  }
  console.info('');
}
