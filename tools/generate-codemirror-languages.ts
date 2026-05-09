#!/usr/bin/env node
import {load as parseYaml} from 'js-yaml';
import {writeFile} from 'node:fs/promises';

const LINGUIST_URL = 'https://raw.githubusercontent.com/github-linguist/linguist/main/lib/linguist/languages.yml';

// Map github-linguist language names to the names CodeMirror's @codemirror/language-data
// uses. Only languages that we want to load with extended extension/filename data are
// listed; everything else falls through to language-data's defaults at runtime.
const linguistToCm: Record<string, string> = {
  'C': 'C',
  'C++': 'C++',
  'C#': 'C#',
  'CMake': 'CMake',
  'COBOL': 'Cobol',
  'CSS': 'CSS',
  'Clojure': 'Clojure',
  'CoffeeScript': 'CoffeeScript',
  'Common Lisp': 'Common Lisp',
  'Crystal': 'Crystal',
  'Cython': 'Cython',
  'D': 'D',
  'Dart': 'Dart',
  'Diff': 'diff',
  'Dockerfile': 'Dockerfile',
  'Elm': 'Elm',
  'Erlang': 'Erlang',
  'F#': 'F#',
  'Fortran': 'Fortran',
  'Go': 'Go',
  'Groovy': 'Groovy',
  'HTML': 'HTML',
  'Haskell': 'Haskell',
  'INI': 'Properties files',
  'JSON': 'JSON',
  'Java': 'Java',
  'JavaScript': 'JavaScript',
  'Julia': 'Julia',
  'Kotlin': 'Kotlin',
  'Less': 'LESS',
  'LiveScript': 'LiveScript',
  'Lua': 'Lua',
  'Markdown': 'Markdown',
  'Nginx': 'Nginx',
  'OCaml': 'OCaml',
  'PHP': 'PHP',
  'Pascal': 'Pascal',
  'Perl': 'Perl',
  'PowerShell': 'PowerShell',
  'Protocol Buffer': 'ProtoBuf',
  'Pug': 'Pug',
  'Puppet': 'Puppet',
  'Python': 'Python',
  'R': 'R',
  'Ruby': 'Ruby',
  'Rust': 'Rust',
  'SCSS': 'SCSS',
  'SQL': 'SQL',
  'Sass': 'Sass',
  'Scala': 'Scala',
  'Scheme': 'Scheme',
  'Shell': 'Shell',
  'Smalltalk': 'Smalltalk',
  'Stylus': 'Stylus',
  'Swift': 'Swift',
  'SystemVerilog': 'SystemVerilog',
  'TOML': 'TOML',
  'TSX': 'TSX',
  'Tcl': 'Tcl',
  'TeX': 'LaTeX',
  'TypeScript': 'TypeScript',
  'VHDL': 'VHDL',
  'Verilog': 'Verilog',
  'Vue': 'Vue',
  'WebAssembly': 'WebAssembly',
  'XML': 'XML',
  'YAML': 'YAML',
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
  const res = await fetch(LINGUIST_URL); // eslint-disable-line no-restricted-globals -- node build script, not browser code
  if (!res.ok) throw new Error(`fetch ${LINGUIST_URL} failed: ${res.status}`);
  const linguist = parseYaml(await res.text()) as Record<string, LinguistEntry>;

  const out: CmLanguage[] = [];
  const missing: string[] = [];
  for (const [linguistName, cmName] of Object.entries(linguistToCm)) {
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
