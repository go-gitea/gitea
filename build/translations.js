#!/usr/bin/env node
import {readFileSync} from 'node:fs';
import {parse} from 'ini';
import {argv} from 'node:process';
import {basename} from 'node:path';

const [cmd] = argv.slice(2);
const cmds = ['dump'];

if (!cmds.includes(cmd)) {
  console.info(`
    Usage: ${basename(argv[1])} command

    Commands:
      dump        Dump all current translation keys to stdout
  `);
}

function dumpObj(obj, path = '') {
  for (const [key, value] of Object.entries(obj)) {
    if (typeof value === 'object' && value !== null) {
      dumpObj(value, path ? `${path}.${key}` : key);
    } else {
      console.info(`${path}.${key}`);
    }
  }
}

if (cmd === 'dump') {
  const text = readFileSync(new URL('../options/locale/locale_en-US.ini', import.meta.url), 'utf8');
  dumpObj(parse(text));
}
