#!/usr/bin/env node
import {load as parseYaml} from 'js-yaml';
import {writeFile} from 'node:fs/promises';
import {languages as cmLanguages} from '@codemirror/language-data';

const linguistUrl = 'https://raw.githubusercontent.com/github-linguist/linguist/main/lib/linguist/languages.yml';

const renames: Record<string, string> = {
  'Protocol Buffer': 'ProtoBuf',
};

// Languages whose entry is constructed manually in the runtime; skip during generation.
const skipNames = new Set(['Dockerfile', 'Markdown']);

// Extensions claimed by several unrelated languages with no good default; strip globally.
const ambiguousExt = new Set(['cgi', 'fcgi', 'inc']);

// Per-language drops for non-text formats (.frm = binary VB6 forms) or where Linguist's
// primary owner conflicts with a more specialised CodeMirror mode (.spec → RPM Spec).
const excludeExt: Record<string, string[]> = {
  'INI': ['frm'],
  'Python': ['spec'],
  'Ruby': ['spec'],
};

// Per-CM-language additions for filenames Linguist classifies as separate languages
// (.editorconfig, .gitconfig, .npmrc) or omits entirely (Snakefile).
const extraFilenames: Record<string, string[]> = {
  'Properties files': ['.editorconfig', '.gitconfig', '.npmrc'],
  'Python': ['Snakefile'],
};

// Per-CM-language additions widely used in practice but absent from Linguist's list.
const extraExtensions: Record<string, string[]> = {
  'Properties files': ['conf'],
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

const res = await fetch(linguistUrl);
if (!res.ok) throw new Error(`fetch ${linguistUrl} failed: ${res.status}`);
const linguist = parseYaml(await res.text()) as Record<string, LinguistEntry>;

const cmByAlias = new Map<string, string>();
// Map of extension -> the CM language that originally owns it. Used to prevent Linguist
// from broadening one language's extension claim into another's territory (e.g. Linguist's
// PLSQL lists .sql, but CM's SQL is the canonical owner).
const cmOriginalExtOwner = new Map<string, string>();
for (const lang of cmLanguages) {
  cmByAlias.set(lang.name.toLowerCase(), lang.name);
  for (const a of lang.alias) cmByAlias.set(a.toLowerCase(), lang.name);
  for (const ext of lang.extensions) {
    if (!cmOriginalExtOwner.has(ext)) cmOriginalExtOwner.set(ext, lang.name);
  }
}

const out: CmLanguage[] = [];
const seen = new Set<string>();
for (const [linguistName, entry] of Object.entries(linguist)) {
  const cmName = renames[linguistName] ?? cmByAlias.get(linguistName.toLowerCase());
  // Multiple Linguist entries can alias to the same CM language (e.g. JSON5 → JSON).
  if (!cmName || skipNames.has(cmName) || seen.has(cmName)) continue;
  seen.add(cmName);
  const exExt = new Set(excludeExt[linguistName]);
  // CodeMirror's matchFilename uses /\.([^.]+)$/, so multi-dot extensions like
  // ".cmake.in" can't match as extensions and are dropped here.
  const extensions = (entry.extensions ?? [])
    .map((e) => e.replace(/^\./, ''))
    .filter((e) => {
      if (e.includes('.') || ambiguousExt.has(e) || exExt.has(e)) return false;
      const owner = cmOriginalExtOwner.get(e);
      return !owner || owner === cmName;
    });
  out.push({
    name: cmName,
    extensions: [...extensions, ...(extraExtensions[cmName] ?? [])],
    filenames: [...(entry.filenames ?? []), ...(extraFilenames[cmName] ?? [])],
  });
}

out.sort((a, b) => a.name.localeCompare(b.name));

const outPath = new URL('../assets/codemirror-languages.json', import.meta.url);
await writeFile(outPath, `${JSON.stringify(out, null, 2)}\n`);
console.info(`wrote ${out.length} languages to ${outPath.pathname}`);
