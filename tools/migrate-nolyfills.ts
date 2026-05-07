#!/usr/bin/env node
// nolyfill writes overrides to package.json#pnpm.overrides which pnpm v11 ignores.
// This moves them to pnpm-workspace.yaml until SukkaW/nolyfill#119 is fixed.
import {readFileSync, writeFileSync} from 'node:fs';
import {exit} from 'node:process';
import {fileURLToPath} from 'node:url';
import {dump} from 'js-yaml';

const packagePath = fileURLToPath(new URL('../package.json', import.meta.url));
const workspacePath = fileURLToPath(new URL('../pnpm-workspace.yaml', import.meta.url));

const packageJson: {pnpm?: {overrides?: Record<string, string>}} = JSON.parse(readFileSync(packagePath, 'utf8'));
const overrides = packageJson.pnpm?.overrides;

if (!overrides || !Object.keys(overrides).length) {
  exit(0);
}

const block = dump({overrides}, {lineWidth: -1, quotingType: "'"});
const workspace = readFileSync(workspacePath, 'utf8');
const overridesRegex = /^overrides:[^\n]*(?:\n(?:[ \t][^\n]*|[ \t]*(?=\n[ \t])))*\n?/m;

if (!overridesRegex.test(workspace)) {
  console.error(`No 'overrides:' block found in pnpm-workspace.yaml`);
  exit(1);
}

writeFileSync(workspacePath, workspace.replace(overridesRegex, block));

const pnpm = packageJson.pnpm!;
delete pnpm.overrides;
if (!Object.keys(pnpm).length) delete packageJson.pnpm;
writeFileSync(packagePath, `${JSON.stringify(packageJson, null, 2)}\n`);
