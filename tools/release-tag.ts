import {execFileSync} from 'node:child_process';
import {appendFileSync, readFileSync} from 'node:fs';
import {parseArgs} from 'node:util';

export function versionTag(value: string): string {
  const version = value.replace(/^v/, '');
  if (!/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(version)) {
    throw new Error('Version must be a stable version, for example 28.0.1 or 29.0.0');
  }
  return `v${version}`;
}

export function validateBranch(branch: string, tag: string): void {
  const match = /^release\/v(0|[1-9]\d*)(?:\.(0|[1-9]\d*))?$/.exec(branch);
  if (!match) throw new Error('Select a release/v* branch when running this workflow');
  const line = match.slice(1).filter((part) => part !== undefined).join('.');
  if (!tag.slice(1).startsWith(`${line}.`)) throw new Error(`Version ${tag} does not belong to ${branch}`);
}

function git(...args: string[]): string {
  return execFileSync('git', args, {encoding: 'utf8'}).trim();
}

function env(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`Missing ${name}`);
  return value;
}

async function api(path: string, token: string, missingOK = false): Promise<Record<string, unknown> | null> {
  const response = await fetch(`https://api.github.com/${path}`, {
    headers: {Authorization: `Bearer ${token}`, Accept: 'application/vnd.github+json'},
    signal: AbortSignal.timeout(30_000),
  });
  if (missingOK && response.status === 404) return null;
  if (!response.ok) throw new Error(`GitHub API check failed: HTTP ${response.status} for ${path}`);
  return await response.json() as Record<string, unknown>;
}

async function authorize(repository: string, token: string): Promise<void> {
  if (!/^[\w.-]+\/[\w.-]+$/.test(repository)) throw new Error('Invalid repository');
  for (const actor of new Set([env('GITHUB_ACTOR'), env('GITHUB_TRIGGERING_ACTOR')])) {
    if (!/^[\w-]+$/.test(actor)) throw new Error('Missing or invalid workflow actor');
    const permission = (await api(`repos/${repository}/collaborators/${actor}/permission`, token))!;
    if (!['maintain', 'admin'].includes(String(permission.role_name))) {
      throw new Error(`${actor} must have the Maintain or Admin repository role`);
    }
  }
}

export function previousRelease(commit: string, tag: string): string {
  const candidates = git('tag', '--merged', commit).split('\n')
    .filter((value) => value !== tag && /^v\d+\.\d+\.\d+$/.test(value));
  if (!candidates.length) throw new Error('No previous release tag is reachable from this branch');
  return git('describe', '--tags', '--abbrev=0', ...candidates.flatMap((value) => ['--match', value]), commit);
}

export async function main(args = process.argv.slice(2)): Promise<void> {
  const {positionals} = parseArgs({args, allowPositionals: true});
  const [command] = positionals;
  if (positionals.length !== 1 || !['prepare', 'preview', 'check', 'range'].includes(command)) {
    throw new Error('Usage: node tools/release-tag.ts prepare|preview|check|range');
  }
  const tag = versionTag(env(command === 'range' ? 'GITHUB_REF_NAME' : 'RELEASE_VERSION'));
  const commit = env('GITHUB_SHA');
  if (command === 'range') {
    if (env('GITHUB_REF_TYPE') !== 'tag' || git('rev-parse', 'HEAD') !== commit) {
      throw new Error('Release notes must be generated from the selected tag commit');
    }
    appendFileSync(env('GITHUB_OUTPUT'), `tag=${tag}\nprevious=${previousRelease(commit, tag)}\n`);
    return;
  }
  const branch = env('GITHUB_REF_NAME');
  if (env('GITHUB_REF_TYPE') !== 'branch') throw new Error('This workflow must run on a release branch');
  validateBranch(branch, tag);
  const repository = env('GITHUB_REPOSITORY');
  const token = env('GITHUB_TOKEN');
  await authorize(repository, token);
  for (const path of [`git/ref/tags/${tag}`, `releases/tags/${tag}`]) {
    if (await api(`repos/${repository}/${path}`, token, true)) {
      throw new Error(`Version ${tag} already exists as a remote tag or GitHub release`);
    }
  }
  if (command !== 'check' && git('rev-parse', 'HEAD') !== commit) {
    throw new Error("Checkout does not match the workflow's selected commit");
  }
  const [remote] = git('ls-remote', 'origin', `refs/heads/${branch}`).split(/\s+/);
  if (remote !== commit) throw new Error('The release branch has moved; start a new workflow run');
  if (command === 'prepare') {
    appendFileSync(env('GITHUB_OUTPUT'), `tag=${tag}\nprevious=${previousRelease(commit, tag)}\n`);
  } else if (command === 'preview') {
    const notes = readFileSync(env('RELEASE_NOTES_FILE'), 'utf8').trim();
    if (!/^ {2}\* \S/m.test(notes)) throw new Error('Generated release notes have no entries');
    appendFileSync(env('GITHUB_STEP_SUMMARY'),
      `## Release preview\n\nBranch: \`${branch}\`\n\nBase commit: \`${commit}\`\n\n` +
      `Tag and commit title: \`${tag}\`\n\nDry-run: \`${env('RELEASE_DRY_RUN')}\`\n\n${notes}\n`);
  } else if (git('rev-parse', 'HEAD^') !== commit || git('rev-parse', 'HEAD^{tree}') !== git('rev-parse', `${commit}^{tree}`)) {
    throw new Error('Release commit must be an empty marker on the selected commit');
  }
}

if (import.meta.main) {
  try {
    await main();
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  }
}
