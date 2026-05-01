import {spawn} from 'node:child_process';

function readTitle() {
  const envTitle = process.env.PR_TITLE?.trim();
  if (envTitle) return envTitle;

  const argvTitle = process.argv
    .slice(2)
    .filter((arg) => arg !== '--')
    .join(' ')
    .trim();
  if (argvTitle) return argvTitle;

  throw new Error('Missing PR title. Provide PR_TITLE env var or pass as CLI args.');
}

const title = readTitle();

const pnpmBin = process.platform === 'win32' ? 'pnpm.cmd' : 'pnpm';
let child;
try {
  child = spawn(pnpmBin, ['exec', 'commitlint'], {
    stdio: ['pipe', 'inherit', 'inherit'],
    env: process.env,
    shell: process.platform === 'win32',
  });
} catch (err) {
  console.error(err);
  process.exit(1);
}

child.stdin.write(`${title}\n`);
child.stdin.end();

child.on('exit', (code) => {
  if (code === 0) process.exit(0);
  console.error(`Invalid PR title: ${title}`);
  process.exit(code ?? 1);
});

child.on('error', (err) => {
  console.error(err);
  process.exit(1);
});
