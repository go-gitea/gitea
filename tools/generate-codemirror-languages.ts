#!/usr/bin/env node
import {load as parseYaml} from 'js-yaml';
import {writeFile} from 'node:fs/promises';
import {languages as cmLanguages} from '@codemirror/language-data';

const linguistUrl = 'https://raw.githubusercontent.com/github-linguist/linguist/main/lib/linguist/languages.yml';

// Linguist names that don't match the corresponding @codemirror/language-data name.
const renames: Record<string, string> = {
  'COBOL': 'Cobol',
  'Diff': 'diff',
  'INI': 'Properties files',
  'Less': 'LESS',
  'Protocol Buffer': 'ProtoBuf',
  'TeX': 'LaTeX',
};

// Per-language extensions to drop. Use only for extensions that would actively collide
// with another language (e.g. .inc claimed by both PHP and C++) or where the syntax is
// genuinely incompatible with the CodeMirror mode (e.g. .csh vs sh).
const excludeExt: Record<string, string[]> = {
  'C++': ['inc'],
  'INI': ['frm'],
  'JavaScript': ['_js', 'bones', 'es', 'es6', 'frag', 'gs', 'jake', 'javascript', 'jsb', 'jscad', 'jsfl', 'jslib', 'jsm', 'jspre', 'jss', 'njs', 'pac', 'sjs', 'ssjs', 'xsjs', 'xsjslib'],
  'Lua': ['fcgi'],
  'PHP': ['fcgi', 'inc'],
  'Perl': ['cgi', 'fcgi'],
  'Python': ['cgi', 'fcgi', 'spec'],
  'Ruby': ['fcgi', 'spec'],
  'Shell': ['cgi', 'csh', 'fcgi'],
  'XML': ['inc', 'jsproj', 'tmpl', 'ts', 'tsx'],
};

type LinguistEntry = {
  type: string;
  extensions?: string[];
  filenames?: string[];
};

type CmLanguage = {
  name: string;
  extensions: string[];
  filenames: string[];
};

async function main() {
  const res = await fetch(linguistUrl); // eslint-disable-line no-restricted-globals -- node build script, not browser code
  if (!res.ok) throw new Error(`fetch ${linguistUrl} failed: ${res.status}`);
  const linguist = parseYaml(await res.text()) as Record<string, LinguistEntry>;

  const cmNames = new Set(cmLanguages.map((l) => l.name));
  const out: CmLanguage[] = [];
  for (const [linguistName, entry] of Object.entries(linguist)) {
    const cmName = renames[linguistName] ?? linguistName;
    if (!cmNames.has(cmName)) continue;
    const exExt = new Set(excludeExt[linguistName]);
    // CodeMirror's matchFilename uses /\.([^.]+)$/ to extract the suffix, so multi-dot
    // extensions like ".cmake.in" cannot match as extensions and are dropped here.
    const extensions = (entry.extensions ?? [])
      .map((e) => e.replace(/^\./, ''))
      .filter((e) => !e.includes('.') && !exExt.has(e));
    const filenames = entry.filenames ?? [];
    out.push({
      name: cmName,
      extensions: Array.from(new Set(extensions)),
      filenames: Array.from(new Set(filenames)),
    });
  }

  out.sort((a, b) => a.name.localeCompare(b.name));

  const outPath = new URL('../assets/codemirror-languages.json', import.meta.url);
  await writeFile(outPath, `${JSON.stringify(out, null, 2)}\n`);
  console.info(`wrote ${out.length} languages to ${outPath.pathname}`);
}

try {
  await main();
} catch (err) {
  console.error(err);
  process.exit(1);
}
