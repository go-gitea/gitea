import {execFileSync} from 'node:child_process';
import type {ExecFileSyncOptionsWithStringEncoding} from 'node:child_process';
import type * as ChildProcess from 'node:child_process';
import {existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {dirname, join} from 'node:path';
import {main, previousRelease, validateBranch, versionTag} from './release-tag.ts';
import {prepare, publish, targets, updateContent, versionParts} from './release-updates.ts';

const state = vi.hoisted(() => ({
  cwd: '',
  repositories: new Map<string, {default_branch: string, ssh_url: string, clone_url: string, html_url: string}>(),
  pulls: new Map<string, {html_url: string, head: {ref: string, repo: {full_name: string}}}[]>(),
  failPR: false,
}));

vi.mock('node:child_process', async (importOriginal) => {
  const original = await importOriginal<typeof ChildProcess>();
  return {...original, execFileSync(file: string, args: string[], options: ExecFileSyncOptionsWithStringEncoding) {
    if (file !== 'tea') return original.execFileSync(file, args, {...options, cwd: options.cwd || state.cwd, stdio: 'pipe'});
    if (args[0] === 'api') {
      const endpoint = args.at(-1)!;
      const repository = endpoint.replace(/^repos\//, '').split('/pulls?')[0];
      return JSON.stringify(endpoint.includes('/pulls?') ? state.pulls.get(repository) || [] : state.repositories.get(repository));
    }
    if (args[0] !== 'pulls' || args[1] !== 'create') throw new Error('Unexpected tea command');
    if (state.failPR) {
      state.failPR = false;
      throw new Error('PR creation failed');
    }
    const repository = args[args.indexOf('--repo') + 1];
    if (state.pulls.has(repository)) throw new Error('Duplicate PR');
    const branch = args[args.indexOf('--head') + 1];
    const url = `https://gitea.com/${repository}/pulls/1`;
    state.pulls.set(repository, [{html_url: url, head: {ref: branch, repo: {full_name: repository}}}]);
    return url;
  }};
});

let root: string;
const git = (...args: string[]) => execFileSync('git', args, {encoding: 'utf8'}).trim();

beforeEach(() => {
  root = mkdtempSync(join(tmpdir(), 'gitea-release-'));
  state.cwd = root;
  state.repositories.clear();
  state.pulls.clear();
  state.failPR = false;
  vi.spyOn(console, 'info').mockImplementation(() => {});
});

afterEach(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  rmSync(root, {recursive: true, force: true});
});

function init(branch: string) {
  git('init', '-b', branch);
  git('config', 'user.name', 'Gitea Release Bot');
  git('config', 'user.email', 'giteabot@users.noreply.github.com');
  git('config', 'commit.gpgsign', 'false');
  git('config', 'tag.gpgsign', 'false');
}

test('validates stable versions and release lines', () => {
  expect(versionTag('v28.0.1')).toBe('v28.0.1');
  expect(() => versionTag('v28.0.1-rc1')).toThrow('stable');
  expect(versionTag('1.27.5')).toBe('v1.27.5');
  validateBranch('release/v28', 'v28.0.1');
  validateBranch('release/v1.27', 'v1.27.5');
  expect(() => validateBranch('main', 'v28.0.1')).toThrow('release/v*');
  expect(() => validateBranch('release/v28', 'v29.0.0')).toThrow('does not belong');
  for (const version of ['028.0.1', '28.0', '28.0.1;echo bad']) expect(() => versionTag(version)).toThrow();
  expect(() => versionParts('28.0.1-rc0')).toThrow('stable');
});

test('previews notes without storing them in commit or tag messages and denies API failures', async () => {
  init('release/v28');
  git('commit', '--allow-empty', '-m', 'initial');
  git('tag', 'v28.0.0');
  git('commit', '--allow-empty', '-m', 'fix: first correction');
  git('tag', 'v28.0.1');
  git('commit', '--allow-empty', '-m', 'fix: second correction');
  git('tag', 'v29.0.0-dev');
  git('tag', 'v28.0.2-rc0');
  const selected = git('rev-parse', 'HEAD');
  const remote = join(root, 'remote.git');
  git('clone', '--bare', root, remote);
  git('remote', 'add', 'origin', remote);
  const notes = '## 28.0.2\n\n* BUGFIXES\n  * Fix: second correction\n';
  const notesFile = join(root, 'release.notes');
  writeFileSync(notesFile, notes);
  for (const [name, value] of Object.entries({
    RELEASE_VERSION: '28.0.2', RELEASE_NOTES_FILE: notesFile, RELEASE_DRY_RUN: 'true',
    GITHUB_REF_TYPE: 'branch', GITHUB_REF_NAME: 'release/v28', GITHUB_SHA: selected,
    GITHUB_REPOSITORY: 'go-gitea/gitea', GITHUB_ACTOR: 'maintainer', GITHUB_TRIGGERING_ACTOR: 'rerunner',
    GITHUB_TOKEN: 'test-token', GITHUB_OUTPUT: join(root, 'output'), GITHUB_STEP_SUMMARY: join(root, 'summary'),
  })) vi.stubEnv(name, value);
  const fetchMock = vi.fn((url: string) => Promise.resolve(url.endsWith('/permission') ?
    Response.json({role_name: 'maintain'}) : new Response('', {status: 404})));
  vi.stubGlobal('fetch', fetchMock);
  await main(['prepare']);
  expect(readFileSync(join(root, 'output'), 'utf8')).toContain('previous=v28.0.1');
  await main(['preview']);
  expect(readFileSync(join(root, 'summary'), 'utf8')).toContain('Fix: second correction');
  expect(existsSync(join(root, 'release.commit'))).toBe(false);
  expect(git('rev-parse', 'HEAD')).toBe(selected);
  expect(git('tag', '--list', 'v28.0.2')).toBe('');
  git('commit', '--allow-empty', '-m', 'v28.0.2');
  git('tag', '-a', '-m', 'v28.0.2', 'v28.0.2');
  await main(['check']);
  expect(git('log', '-1', '--format=%B')).toBe('v28.0.2');
  expect(git('for-each-ref', '--format=%(contents)', 'refs/tags/v28.0.2')).toBe('v28.0.2');
  const marker = git('rev-parse', 'HEAD');
  expect(previousRelease(marker, 'v28.0.2')).toBe('v28.0.1');
  vi.stubEnv('GITHUB_REF_TYPE', 'tag');
  vi.stubEnv('GITHUB_REF_NAME', 'v28.0.2');
  vi.stubEnv('GITHUB_SHA', marker);
  await main(['range']);
  expect(readFileSync(join(root, 'output'), 'utf8').split('previous=v28.0.1')).toHaveLength(3);
  vi.stubEnv('GITHUB_REF_TYPE', 'branch');
  vi.stubEnv('GITHUB_REF_NAME', 'release/v28');
  vi.stubEnv('GITHUB_SHA', selected);
  fetchMock.mockImplementation(() => Promise.resolve(new Response('', {status: 500})));
  await expect(main(['check'])).rejects.toThrow('HTTP 500');
  fetchMock.mockImplementation(() => Promise.resolve(Response.json({role_name: 'write'})));
  await expect(main(['check'])).rejects.toThrow('Maintain or Admin');
  fetchMock.mockImplementation((url: string) => Promise.resolve(url.endsWith('/permission') ?
    Response.json({role_name: 'admin'}) : Response.json({name: 'existing'})));
  await expect(main(['check'])).rejects.toThrow('already exists');
});

const contents = {
  deployment: '{"latest":{"version":"28.0.0"},"other":{"version":"1.0.0"}}\n',
  'helm-gitea': 'version: 0.0.0\nappVersion: "28.0.0" # stable\n',
  'terraform-provider-gitea': 'services:\n  server:\n    image: gitea/gitea:28.0.0\n',
};

test('updates only the release fields, skips equal or newer versions, and rejects ambiguous input', () => {
  for (const target of Object.keys(targets) as (keyof typeof targets)[]) {
    const updated = updateContent(target, contents[target], '28.0.1');
    expect(updated).toBe(contents[target].replace('28.0.0', '28.0.1'));
    expect(updateContent(target, updated, '28.0.1')).toBe(updated);
    expect(updateContent(target, updated, '1.27.5')).toBe(updated);
    expect(updateContent(target, updated, '28.0.10')).toContain('28.0.10');
  }
  for (const content of ['appVersion: latest\n', 'appVersion: 28.0.0\nappVersion: 28.0.0\n']) {
    expect(() => updateContent('helm-gitea', content, '28.0.1')).toThrow('exactly one');
  }
});

test('publishes local Git updates, recovers after PR failure, and reuses existing commits and PRs', () => {
  const workdir = join(root, 'updates');
  mkdirSync(workdir);
  for (const target of Object.keys(targets) as (keyof typeof targets)[]) {
    state.cwd = join(root, 'source', target);
    mkdirSync(state.cwd, {recursive: true});
    init('main');
    const path = join(state.cwd, targets[target]);
    mkdirSync(dirname(path), {recursive: true});
    writeFileSync(path, contents[target]);
    git('add', '.');
    git('commit', '-m', 'initial');
    const remote = join(root, `${target}.git`);
    git('clone', '--bare', state.cwd, remote);
    state.repositories.set(`gitea/${target}`, {
      default_branch: 'main', ssh_url: remote, clone_url: remote, html_url: `https://gitea.com/gitea/${target}`,
    });
  }
  for (const target of Object.keys(targets) as (keyof typeof targets)[]) {
    let plan = prepare('test', workdir, target, '28.0.1');
    state.cwd = plan.checkout;
    git('config', 'commit.gpgsign', 'false');
    expect(readFileSync(join(plan.checkout, plan.filename), 'utf8')).toBe(plan.old);
    expect(git('status', '--porcelain')).toBe('');
    if (target === 'deployment') {
      state.failPR = true;
      expect(() => publish('test', plan)).toThrow('PR creation failed');
      const head = git('rev-parse', 'HEAD');
      plan = prepare('test', workdir, target, '28.0.1');
      expect(plan.head).toBe(head);
    }
    publish('test', plan);
    const head = git('rev-parse', 'HEAD');
    expect(readFileSync(join(plan.checkout, plan.filename), 'utf8')).toBe(plan.updated);
    plan = prepare('test', workdir, target, '28.0.1');
    publish('test', plan);
    expect(git('rev-parse', 'HEAD')).toBe(head);
  }
  expect(state.pulls.size).toBe(3);
  state.cwd = join(root, 'source', 'deployment');
  writeFileSync(join(state.cwd, 'version.json'), contents.deployment.replace('28.0.0', '28.0.2'));
  git('commit', '-am', 'newer release');
  git('push', state.repositories.get('gitea/deployment')!.ssh_url, 'main');
  expect(prepare('test', workdir, 'deployment', '28.0.1').active).toBe(false);
  writeFileSync(join(workdir, 'deployment', 'unrelated.txt'), 'dirty');
  expect(() => prepare('test', workdir, 'deployment', '28.0.1')).toThrow('clean');
});
