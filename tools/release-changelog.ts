import {execFileSync} from 'node:child_process';
import {mkdtempSync, readFileSync, rmSync, writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {argv, env, exit} from 'node:process';
import {pathToFileURL} from 'node:url';
import {load} from 'js-yaml';

type Group = {name: string, labels?: string[], default?: boolean};
type Config = {repo: string, groups: Group[], 'skip-labels': string};
type Commit = {id: string, message: string, raw_message?: string, group?: string};
type Release = {version: string, timestamp: number, commits: Commit[]};
type Pull = {number: number, title: string, merged_at: string | null, labels: {name: string}[], base: {ref: string}};

function releaseVersion(value: string): string {
  const version = value.replace(/^v/, '');
  if (!/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-rc\d+)?$/.test(version)) {
    throw new Error('RELEASE_VERSION must be a stable or rc version, for example 28.0.1 or 29.0.0-rc0');
  }
  return version;
}

function classifyCommit(commit: Commit, pull: Pull | undefined, config: Config): Commit[] {
  const subject = (commit.raw_message || commit.message).split('\n')[0];
  if (/^(?:chore|ci)(?:\([^)]*\))?!?:/.test(subject) || subject === '[skip ci] Updated translations via Crowdin') return [];
  if (/\(#\d+\)$/.test(subject) && !pull) throw new Error(`Missing PR metadata for ${commit.id}`);
  const labels = pull ? pull.labels.map(({name}) => name) : [];
  const skip = new RegExp(config['skip-labels']);
  // Release ranges already exclude duplicate backports; scheduling labels must not hide shipped fixes.
  if (labels.some((label) => skip.test(label) && !(pull!.base.ref.startsWith('release/v') && label.startsWith('backport/')))) return [];
  const groups = config.groups.filter((group) => (group.labels || []).some((label) => labels.includes(label)));
  const highlights = groups.filter(({name}) => name === 'BREAKING' || name === 'SECURITY');
  const selected = highlights.length ? highlights : [groups[0] || config.groups.find((group) => group.default)!];
  const message = pull ?
    `${subject.replace(/\s*\(#\d+\)$/, '')} ([#${pull.number}](https://github.com/${config.repo}/pull/${pull.number}))` :
    subject;
  return selected.map((group) => ({...commit, message, raw_message: message, group: `${config.groups.indexOf(group).toString().padStart(2, '0')}:${group.name}`}));
}

function prependRelease(changelog: string, section: string, version: string): string {
  const headings = changelog.matchAll(/^## (.+)$/gm).toArray();
  if (headings.some((heading) => new RegExp(`^\\[?${version.replaceAll('.', '\\.')}[\\] ]`).test(heading[1]))) {
    throw new Error(`CHANGELOG.md already contains ${version}`);
  }
  if (!headings.length) throw new Error('CHANGELOG.md has no release headings');
  const offset = headings[0].index;
  return `${changelog.slice(0, offset)}${section.trim()}\n\n${changelog.slice(offset)}`;
}

function previousReleaseTag(): string {
  const tags = git('tag', '--merged', 'HEAD').split('\n').filter((tag) => /^v\d+\.\d+\.\d+(?:-rc\d+)?$/.test(tag));
  if (!tags.length) throw new Error('No release tag is reachable from HEAD; set RELEASE_PREVIOUS explicitly');
  return git('describe', '--tags', '--abbrev=0', ...tags.flatMap((tag) => ['--match', tag]), 'HEAD');
}

function git(...args: string[]): string {
  return execFileSync('git', args, {encoding: 'utf8'}).trim();
}

function changelogPullRequest(version: string, base: string, head: string): {title: string, body: string, base: string, head: string} {
  return {
    title: `docs(changelog): prepare v${version}`,
    body: `Prepare the changelog for v${version}. After this PR is reviewed and merged into \`${base}\`, a release maintainer will run \`release-create-tag\` with dry-run enabled, review the preview, and then run it with dry-run disabled to sign and push the tag and start the release pipeline.`,
    base,
    head,
  };
}

async function submitChangelogPullRequest(repo: string, proposal: ReturnType<typeof changelogPullRequest>): Promise<string> {
  if (!env.GITHUB_TOKEN) throw new Error('Set GITHUB_TOKEN with permission to create pull requests');
  const headers = {Accept: 'application/vnd.github+json', Authorization: `Bearer ${env.GITHUB_TOKEN}`};
  const query = new URLSearchParams({state: 'open', head: `${repo.split('/')[0]}:${proposal.head}`, base: proposal.base});
  const url = `https://api.github.com/repos/${repo}/pulls`;
  const response = await fetch(`${url}?${query}`, {headers});
  if (!response.ok) throw new Error(`Unable to check existing PRs: HTTP ${response.status}`);
  const existing = await response.json() as {html_url: string}[];
  if (existing.length) return `Changelog PR already exists: ${existing[0].html_url}`;
  const created = await fetch(url, {method: 'POST', headers: {...headers, 'Content-Type': 'application/json'}, body: JSON.stringify(proposal)});
  if (!created.ok) throw new Error(`Unable to create changelog PR: HTTP ${created.status}`);
  return `Created changelog PR: ${(await created.json() as {html_url: string}).html_url}`;
}

async function createPullRequest(): Promise<void> {
  const version = releaseVersion(env.RELEASE_VERSION || '');
  const base = env.RELEASE_BRANCH || '';
  const line = /^release\/v(\d+)(?:\.(\d+))?$/.exec(base);
  if (!line || !version.startsWith(`${line.slice(1).filter(Boolean).join('.')}.`)) throw new Error('RELEASE_BRANCH must be the release branch for RELEASE_VERSION');
  const branch = git('branch', '--show-current');
  if (!branch || branch === base || branch === 'main' || branch.startsWith('release/v')) throw new Error('Run this command on the changelog preparation branch');
  if (git('status', '--porcelain', '--untracked-files=no')) throw new Error('Commit the changelog before creating the PR');
  const config = load(readFileSync('.changelog.yml', 'utf8')) as Config;
  const remote = /^https:\/\/github\.com\/([\w.-]+\/[\w.-]+?)(?:\.git)?$|^git@github\.com:([\w.-]+\/[\w.-]+?)(?:\.git)?$/.exec(git('remote', 'get-url', 'origin'));
  if (!remote || (remote[1] || remote[2]) !== config.repo) throw new Error(`origin must point to ${config.repo}`);
  const remoteHead = git('ls-remote', 'origin', `refs/heads/${branch}`).split(/\s+/)[0];
  if (remoteHead !== git('rev-parse', 'HEAD')) throw new Error('Push the changelog branch to origin before creating its PR');
  git('fetch', 'origin', base);
  if (git('diff', '--name-only', 'FETCH_HEAD...HEAD') !== 'CHANGELOG.md') throw new Error('The PR must change only CHANGELOG.md relative to the release branch');
  const changelog = readFileSync('CHANGELOG.md', 'utf8');
  const heading = /^## .+$/m.exec(changelog);
  if (!heading || !heading[0].startsWith(`## ${version} - `)) throw new Error('The selected version must be the first changelog release');
  const proposal = changelogPullRequest(version, base, branch);
  if (env.RELEASE_PR_DRY_RUN !== 'false') {
    console.info(`Target: https://github.com/${config.repo}/compare/${encodeURIComponent(base)}...${encodeURIComponent(branch)}`);
    console.info(`Title: ${proposal.title}\n\n${proposal.body}`);
    console.info('Preview only. To create this PR, rerun with RELEASE_PR_DRY_RUN=false after reviewing the target and text.');
    return;
  }
  console.info(await submitChangelogPullRequest(config.repo, proposal));
}

async function generateChangelog(): Promise<void> {
  const version = releaseVersion(env.RELEASE_VERSION || '');
  const previous = env.RELEASE_PREVIOUS || previousReleaseTag();
  if (!/^v\d+\.\d+\.\d+(?:-rc\d+)?$/.test(previous)) throw new Error('RELEASE_PREVIOUS must name the previous release tag');
  if (git('status', '--porcelain', '--untracked-files=no')) throw new Error('Commit or stash tracked changes before preparing the changelog');
  if (git('tag', '--list', `v${version}`)) throw new Error(`Tag v${version} already exists`);
  git('rev-parse', '--verify', `refs/tags/${previous}^{commit}`);
  git('merge-base', '--is-ancestor', `refs/tags/${previous}`, 'HEAD');
  console.info(`Generating ${version} from ${previous}..HEAD`);
  const config = load(readFileSync('.changelog.yml', 'utf8')) as Config;
  if (!/^[\w.-]+\/[\w.-]+$/.test(config.repo)) throw new Error('Invalid repository in .changelog.yml');
  const changelog = readFileSync('CHANGELOG.md', 'utf8');
  prependRelease(changelog, '', version);
  const releases = JSON.parse(execFileSync('git-cliff', [
    '--config', 'cliff.toml', '--offline', '--context', '--tag', `v${version}`, `${previous}..HEAD`,
  ], {encoding: 'utf8', maxBuffer: 64 * 1024 * 1024})) as Release[];
  const release = releases[0];
  if (releases.length !== 1 || !release.commits.length) throw new Error('The selected range must contain unreleased commits');
  const date = env.RELEASE_DATE || new Date().toISOString().slice(0, 10);
  const timestamp = Date.parse(`${date}T00:00:00Z`);
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date) || !Number.isFinite(timestamp) || new Date(timestamp).toISOString().slice(0, 10) !== date) {
    throw new Error('RELEASE_DATE must be a valid YYYY-MM-DD date');
  }
  release.timestamp = timestamp / 1000;
  const commits: Commit[] = [];
  const seen = new Set<number>();
  for (const commit of release.commits) {
    const subject = (commit.raw_message || commit.message).split('\n')[0];
    const match = /\(#(\d+)\)$/.exec(subject);
    let pull: Pull | undefined;
    if (match && !/^(?:chore|ci)(?:\([^)]*\))?!?:/.test(subject)) {
      const number = Number(match[1]);
      if (seen.has(number)) continue;
      seen.add(number);
      const response = await fetch(`https://api.github.com/repos/${config.repo}/pulls/${number}`, {
        headers: {
          Accept: 'application/vnd.github+json',
          ...(env.GITHUB_TOKEN && {Authorization: `Bearer ${env.GITHUB_TOKEN}`}),
        },
      });
      if (!response.ok) throw new Error(`Unable to fetch PR ${number}: HTTP ${response.status}; set GITHUB_TOKEN for authenticated requests`);
      pull = await response.json() as Pull;
      if (!pull.merged_at) throw new Error(`PR ${number} is not merged`);
    }
    commits.push(...classifyCommit(commit, pull, config));
  }
  release.commits = commits;
  if (!commits.length) throw new Error('No changelog entries remain after filtering');
  const directory = mkdtempSync(join(tmpdir(), 'gitea-changelog-'));
  try {
    const context = join(directory, 'context.json');
    writeFileSync(context, JSON.stringify(releases), {mode: 0o600});
    const section = execFileSync('git-cliff', ['--config', 'cliff.toml', '--offline', '--from-context', context], {encoding: 'utf8'});
    writeFileSync('CHANGELOG.md', prependRelease(changelog, section, version));
  } finally {
    rmSync(directory, {recursive: true, force: true});
  }
  console.info(`Prepared ${version} in CHANGELOG.md. Review it, commit it on a branch, and push the branch for review.`);
}

if (argv[1] && import.meta.url === pathToFileURL(argv[1]).href) {
  try {
    if (argv[2] === 'pr') await createPullRequest();
    else await generateChangelog();
  } catch (error) {
    console.error(error instanceof Error ? error.message : error);
    exit(1);
  }
}
