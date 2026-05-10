#!/usr/bin/env node
import {load as parseYaml} from 'js-yaml';
import {writeFile} from 'node:fs/promises';
import {languages as cmLanguages} from '@codemirror/language-data';

const linguistUrl = 'https://raw.githubusercontent.com/github-linguist/linguist/main/lib/linguist/languages.yml';

// Linguist names not resolvable through CodeMirror's name+alias matching below.
const renames: Record<string, string> = {
  'Protocol Buffer': 'ProtoBuf',
};

// Extensions claimed by several unrelated languages with no good default winner.
// Strip globally so files with these suffixes fall through to plain text.
const ambiguousExt = new Set(['cgi', 'fcgi', 'inc']);

// Per-language extensions to drop where the file isn't a text format (.frm is binary
// VB6 form data) or where Linguist's primary owner conflicts with a more specialised
// CodeMirror mode in our set (.spec → RPM Spec rather than Python/RSpec).
const excludeExt: Record<string, string[]> = {
  'INI': ['frm'],
  'Python': ['spec'],
  'Ruby': ['spec'],
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
  const res = await fetch(linguistUrl);
  if (!res.ok) throw new Error(`fetch ${linguistUrl} failed: ${res.status}`);
  const linguist = parseYaml(await res.text()) as Record<string, LinguistEntry>;

  // Map every CM name and alias (lowercased) to its canonical CM name.
  const cmByAlias = new Map<string, string>();
  for (const lang of cmLanguages) {
    cmByAlias.set(lang.name.toLowerCase(), lang.name);
    for (const a of lang.alias) cmByAlias.set(a.toLowerCase(), lang.name);
  }

  const out: CmLanguage[] = [];
  const seen = new Set<string>();
  for (const [linguistName, entry] of Object.entries(linguist)) {
    const cmName = renames[linguistName] ?? cmByAlias.get(linguistName.toLowerCase());
    // Multiple Linguist entries can alias to the same CM language (e.g. JSON5 → JSON);
    // keep the first to avoid duplicate descriptions in the runtime list.
    if (!cmName || seen.has(cmName)) continue;
    seen.add(cmName);
    const exExt = new Set(excludeExt[linguistName]);
    // CodeMirror's matchFilename uses /\.([^.]+)$/ to extract the suffix, so multi-dot
    // extensions like ".cmake.in" cannot match as extensions and are dropped here.
    const extensions = (entry.extensions ?? [])
      .map((e) => e.replace(/^\./, ''))
      .filter((e) => !e.includes('.') && !ambiguousExt.has(e) && !exExt.has(e));
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
