import {execFileSync, spawnSync} from 'node:child_process';
import {existsSync, mkdirSync, readFileSync, writeFileSync} from 'node:fs';
import {join, resolve} from 'node:path';
import {createInterface} from 'node:readline/promises';
import {parseArgs} from 'node:util';

export const targets = {
  deployment: 'version.json',
  'helm-gitea': 'Chart.yaml',
  'terraform-provider-gitea': 'scripts/docker-compose.yaml',
};
type Target = keyof typeof targets;
type Pull = {html_url: string, head: {ref: string, repo: {full_name: string}}};
type Repository = {default_branch: string, ssh_url: string, clone_url: string, html_url: string};
type Plan = {
  checkout: string, repository: string, branch: string, base: string, filename: string,
  old: string, updated: string, title: string, body: string, active: boolean, head: string,
};

export function versionParts(value: string): number[] {
  if (!/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(value)) {
    throw new Error(`Expected a stable release version, got ${JSON.stringify(value)}`);
  }
  return value.split('.').map(Number);
}

function newer(version: string, current: string): boolean {
  const existing = versionParts(current);
  for (const [index, part] of versionParts(version).entries()) {
    if (part !== existing[index]) return part > existing[index];
  }
  return false;
}

export function updateContent(target: Target, content: string, version: string): string {
  if (target === 'deployment') {
    const data = JSON.parse(content) as {latest: {version: string}};
    if (!newer(version, data.latest.version)) return content;
    data.latest.version = version;
    return `${JSON.stringify(data, null, content.trimEnd().includes('\n') ? 2 : undefined)}\n`;
  }
  const pattern = target === 'helm-gitea' ?
    /^(appVersion:[\t ]*)(["']?)(\d+\.\d+\.\d+)\2([\t ]*(?:#.*)?)$/gm :
    /^([\t ]*image:[\t ]*gitea\/gitea:)(["']?)(\d+\.\d+\.\d+)\2([\t ]*(?:#.*)?)$/gm;
  const matches = Array.from(content.matchAll(pattern));
  if (matches.length !== 1) throw new Error(`Expected exactly one release version in ${targets[target]}`);
  const match = matches[0];
  if (!newer(version, match[3])) return content;
  const start = match.index + match[1].length + match[2].length;
  return content.slice(0, start) + version + content.slice(start + match[3].length);
}

function run(command: string[], cwd?: string): string {
  return execFileSync(command[0], command.slice(1), {cwd, encoding: 'utf8'}).trim();
}

function api<T>(login: string, endpoint: string): T {
  return JSON.parse(run(['tea', 'api', '--login', login, endpoint])) as T;
}

function findPR(login: string, repository: string, branch: string): string | undefined {
  for (let page = 1; ; page++) {
    const pulls = api<Pull[]>(login, `repos/${repository}/pulls?state=open&limit=50&page=${page}`);
    const pull = pulls.find((value) => value.head.ref === branch && value.head.repo.full_name === repository);
    if (pull) return pull.html_url;
    if (pulls.length < 50) return undefined;
  }
}

export function prepare(login: string, workdir: string, target: Target, version: string): Plan {
  const repository = `gitea/${target}`;
  const metadata = api<Repository>(login, `repos/${repository}`);
  const checkout = join(workdir, target);
  const branch = `release/update-${version}`;
  const base = metadata.default_branch;
  if (!existsSync(checkout)) run(['git', 'clone', metadata.ssh_url, checkout]);
  const git = (...args: string[]) => run(['git', ...args], checkout);
  if (![metadata.ssh_url, metadata.clone_url].includes(git('remote', 'get-url', 'origin'))) {
    throw new Error(`Unexpected origin in ${checkout}`);
  }
  if (git('status', '--porcelain')) throw new Error(`Checkout must be clean: ${checkout}`);
  git('fetch', 'origin');
  const hasRef = (ref: string) => git('for-each-ref', '--format=%(refname)', ref).split('\n').includes(ref);
  const remoteBranch = hasRef(`refs/remotes/origin/${branch}`);
  if (hasRef(`refs/heads/${branch}`)) {
    git('checkout', branch);
    if (remoteBranch) git('merge', '--ff-only', `origin/${branch}`);
  } else {
    git('checkout', '-b', branch, `origin/${remoteBranch ? branch : base}`);
  }
  const filename = targets[target];
  const title = `chore(deps): update to Gitea ${version}`;
  const commits = git('log', '--format=%s', `origin/${base}..HEAD`).split('\n').filter(Boolean);
  const changed = git('diff', '--name-only', `origin/${base}...HEAD`).split('\n').filter(Boolean);
  if (commits.some((subject) => subject !== title) || changed.some((path) => path !== filename)) {
    throw new Error(`Release branch contains unrelated changes: ${checkout}`);
  }
  const old = readFileSync(join(checkout, filename), 'utf8');
  const updated = updateContent(target, old, version);
  const baseContent = `${git('show', `origin/${base}:${filename}`)}\n`;
  const eligible = updateContent(target, baseContent, version) !== baseContent;
  const body = target === 'deployment' ? `Update the latest stable Gitea version to ${version}.` :
    `Update the Gitea ${target === 'helm-gitea' ? 'app version' : 'test image'} to ${version}.`;
  console.info(`\nTarget: ${metadata.html_url} (${branch} → ${base})\nTitle: ${title}\nBody: ${body}`);
  if (eligible) {
    console.info(git('diff', `origin/${base}...HEAD`, '--', filename));
    const diff = spawnSync('git', ['diff', '--no-index', '--', filename, '-'], {cwd: checkout, encoding: 'utf8', input: updated});
    if (diff.error) throw diff.error;
    if (diff.status !== 0 && diff.status !== 1) throw new Error(diff.stderr);
    console.info(diff.stdout);
  }
  const pull = findPR(login, repository, branch);
  const active = eligible && (old !== updated || commits.length > 0);
  console.info(pull ? `Existing PR: ${pull}` : active ? 'Will open a PR.' : 'Already current or newer; skipping.');
  return {checkout, repository, branch, base, filename, old, updated, title, body, active, head: git('rev-parse', 'HEAD')};
}

export function publish(login: string, plan: Plan): void {
  const git = (...args: string[]) => run(['git', ...args], plan.checkout);
  const path = join(plan.checkout, plan.filename);
  if (git('status', '--porcelain') || readFileSync(path, 'utf8') !== plan.old ||
      git('rev-parse', 'HEAD') !== plan.head || git('branch', '--show-current') !== plan.branch) {
    throw new Error(`Checkout changed since preview: ${plan.checkout}`);
  }
  if (plan.updated !== plan.old) {
    writeFileSync(path, plan.updated);
    git('diff', '--check');
    git('add', '--', plan.filename);
    git('-c', 'user.name=Gitea Release Bot', '-c', 'user.email=giteabot@users.noreply.github.com', 'commit', '-m', plan.title);
  }
  git('push', '--set-upstream', 'origin', plan.branch);
  const pull = findPR(login, plan.repository, plan.branch);
  console.info(pull || run(['tea', 'pulls', 'create', '--login', login, '--repo', plan.repository,
    '--head', plan.branch, '--base', plan.base, '--title', plan.title, '--description', plan.body], plan.checkout));
}

async function main(): Promise<void> {
  const {values, positionals} = parseArgs({allowPositionals: true, options: {
    workdir: {type: 'string'}, login: {type: 'string', default: 'gitea.com'},
    publish: {type: 'boolean', default: false}, help: {type: 'boolean', short: 'h'},
  }});
  if (values.help) {
    console.info('Usage: node tools/release-updates.ts VERSION --workdir DIRECTORY [--login LOGIN] [--publish]');
    return;
  }
  if (positionals.length !== 1 || !values.workdir) throw new Error('Specify a stable VERSION and --workdir DIRECTORY');
  const version = positionals[0].replace(/^v/, '');
  versionParts(version);
  const workdir = resolve(values.workdir);
  mkdirSync(workdir, {recursive: true});
  const plans = (Object.keys(targets) as Target[]).map((target) => prepare(values.login, workdir, target, version));
  const active = plans.filter((plan) => plan.active);
  if (!values.publish || !active.length) return;
  const prompt = createInterface({input: process.stdin, output: process.stdout});
  let answer: string;
  try {
    answer = await prompt.question('\nCommit, push and open the PRs shown above? Type yes: ');
  } finally {
    prompt.close();
  }
  if (answer !== 'yes') {
    console.info('Publication cancelled.');
    return;
  }
  for (const plan of active) publish(values.login, plan);
}

if (import.meta.main) {
  try {
    await main();
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  }
}
