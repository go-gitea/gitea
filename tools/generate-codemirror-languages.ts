#!/usr/bin/env node
import {load as parseYaml} from 'js-yaml';
import {writeFile} from 'node:fs/promises';

const linguistUrl = 'https://raw.githubusercontent.com/github-linguist/linguist/main/lib/linguist/languages.yml';

// Languages to extract from github-linguist. A bare string means the linguist name
// matches CodeMirror's @codemirror/language-data name; a tuple is [linguist, cm] when
// they differ. Anything not listed falls through to language-data's defaults at runtime.
const languages: Array<string | [string, string]> = [
  'C', 'C++', 'C#', 'CMake', ['COBOL', 'Cobol'], 'CSS', 'Clojure', 'CoffeeScript',
  'Common Lisp', 'Crystal', 'Cython', 'D', 'Dart', ['Diff', 'diff'], 'Dockerfile',
  'Elm', 'Erlang', 'F#', 'Fortran', 'Go', 'Groovy', 'HTML', 'Haskell',
  ['INI', 'Properties files'], 'JSON', 'Java', 'JavaScript', 'Julia', 'Kotlin',
  ['Less', 'LESS'], 'LiveScript', 'Lua', 'Markdown', 'Nginx', 'OCaml', 'PHP', 'Pascal',
  'Perl', 'PowerShell', ['Protocol Buffer', 'ProtoBuf'], 'Pug', 'Puppet', 'Python', 'R',
  'Ruby', 'Rust', 'SCSS', 'SQL', 'Sass', 'Scala', 'Scheme', 'Shell', 'Smalltalk',
  'Stylus', 'Swift', 'SystemVerilog', 'TOML', 'TSX', 'Tcl', ['TeX', 'LaTeX'],
  'TypeScript', 'VHDL', 'Verilog', 'Vue', 'WebAssembly', 'XML', 'YAML',
];

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

  const out: CmLanguage[] = [];
  const missing: string[] = [];
  for (const lang of languages) {
    const [linguistName, cmName] = typeof lang === 'string' ? [lang, lang] : lang;
    const entry = linguist[linguistName];
    if (!entry) {
      missing.push(linguistName);
      continue;
    }
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

  if (missing.length) {
    console.warn(`linguist entries not found: ${missing.join(', ')}`);
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
