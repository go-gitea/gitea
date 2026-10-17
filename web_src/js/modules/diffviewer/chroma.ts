import {globCompile} from '../../utils/glob.ts';
import type {Segment} from './highlight.ts';

// Runs the regex lexers of chroma (the server-side highlighter) for the languages without a CodeMirror grammar,
// they are converted to JSON by build/generate-chroma-lexers.go.

type Emitter = string | null | {g: Emitter[]} | {u: string} | {s: string} | {ug: [nameGroup: number, codeGroup: number, emitters: Emitter[]]};
type Mutator = null | {push: string[] | null} | {pop: number} | {include: string} | {combined: string[]} | Mutator[];
type LexerFile = {name: string; flags: string; ensureNL?: boolean; rules: Record<string, Array<[string, Emitter, Mutator]>>; using?: string[]};
type IndexEntry = {name: string; file: string; aliases?: string[]; filenames?: string[]};
type LexerEntry = IndexEntry & {patterns: RegExp[]; patternsIgnoreCase: RegExp[]};
type Rule = {re: RegExp | null; emitter: Emitter; mutator: Mutator};

const lexerFiles = import.meta.glob<LexerFile>('../../../../assets/chroma-lexers/*.json', {import: 'default'});
const maxZeroLengthMatches = 100;
const tokenizeTimeBudget = 1000; // ms, JavaScript has no match timeout like regexp2, slow patterns must not block the page

// the syntax characters which keep their backslash in unicode mode, and their escapes inside a character class
const syntaxChars = new Set('^$\\.*+?()[]{}|/');
const classSyntaxChars = new Set('\\]^-');

// without unicode mode, unicode properties are approximated by ranges
function unicodePropertyRange(property: string): string {
  return property.startsWith('N') ? '0-9' : 'a-zA-Z\\u00C0-\\uFFFF';
}

// translates a pattern of regexp2 (.NET style, used by chroma) to a JavaScript regular expression, unicode mode if possible
export function translatePattern(pattern: string, unicode: boolean): string {
  let src = pattern;
  let extended = false;
  for (let leading = /^\(\?([imsx]+)\)/.exec(src); leading; leading = /^\(\?([imsx]+)\)/.exec(src)) {
    extended ||= leading[1].includes('x');
    src = src.slice(leading[0].length);
  }
  let out = '';
  let inClass = false;
  for (let i = 0; i < src.length; i++) {
    const ch = src[i];
    if (ch === '\\') {
      const next = src[i + 1] ?? '';
      i++;
      if (!unicode && (next === 'p' || next === 'P') && src[i + 1] === '{') {
        const end = src.indexOf('}', i);
        const range = unicodePropertyRange(src.slice(i + 2, end));
        out += inClass ? range : `[${next === 'P' ? '^' : ''}${range}]`;
        i = end;
        continue;
      }
      if (inClass) {
        if (/[a-zA-Z0-9]/.test(next) || classSyntaxChars.has(next)) out += `\\${next}`;
        else out += next;
        continue;
      }
      if (next === 'A') out += '^';
      else if (next === 'Z' || next === 'z') out += '$';
      else if (next === 'h') out += '[ \\t]';
      else if (next === 'e') out += '\\x1b';
      else if (next === 'x' && src[i + 1] === '{') {
        const end = src.indexOf('}', i);
        out += unicode ? `\\u{${src.slice(i + 2, end)}}` : `\\u${src.slice(i + 2, end).padStart(4, '0')}`;
        i = end;
      } else if (next === 'Q') {
        const end = src.indexOf('\\E', i);
        const literal = src.slice(i + 1, end === -1 ? src.length : end);
        out += literal.replace(/[\\^$.*+?()[\]{}|/]/g, '\\$&');
        i = end === -1 ? src.length : end + 1;
      } else if (/[a-zA-Z0-9]/.test(next) || syntaxChars.has(next)) out += `\\${next}`;
      else out += next;
      continue;
    }
    if (inClass) {
      if (ch === ']') inClass = false;
      out += ch;
      continue;
    }
    if (ch === '[') {
      inClass = true;
      out += ch;
      if (src[i + 1] === '^') out += src[++i];
      if (src[i + 1] === ']') out += `\\${src[++i]}`; // a leading "]" is a literal in .NET and Go
      continue;
    }
    if (extended && /\s/.test(ch)) continue;
    if (extended && ch === '#') {
      while (i < src.length && src[i] !== '\n') i++;
      continue;
    }
    if (ch === '(' && src.startsWith('(?#', i)) {
      i = src.indexOf(')', i);
      continue;
    }
    if (ch === '(' && src.startsWith('(?>', i)) {
      out += '(?:'; // atomic groups don't exist in JavaScript, a non-capturing group is the closest
      i += 2;
      continue;
    }
    if (ch === '(' && src.startsWith('(?P<', i)) {
      out += '(?<';
      i += 3;
      continue;
    }
    if ((ch === '+') && /[*+?}]$/.test(out) && !out.endsWith('\\+') && !out.endsWith('\\*') && !out.endsWith('\\?') && !out.endsWith('\\}')) {
      continue; // possessive quantifier
    }
    out += ch;
  }
  return out;
}

function compilePattern(pattern: string, flags: string): RegExp | null {
  const leadingFlags = /^\(\?([imsx]+)\)/.exec(pattern)?.[1] ?? '';
  const allFlags = [...new Set(`${flags}${leadingFlags.replace('x', '')}y`)].join('');
  for (const unicode of [true, false]) {
    try {
      return new RegExp(translatePattern(pattern, unicode), unicode ? `${allFlags}u` : allFlags);
    } catch {}
  }
  return null; // the rule never matches, like a pattern with a feature JavaScript doesn't support
}

export class ChromaLexer {
  name: string;
  rules = new Map<string, Rule[]>();
  ensureNL: boolean;
  resolve: (name: string) => ChromaLexer | null;

  constructor(file: LexerFile, resolve: (name: string) => ChromaLexer | null) {
    this.name = file.name;
    this.ensureNL = Boolean(file.ensureNL);
    this.resolve = resolve;
    const raw = new Map(Object.entries(file.rules).map(([state, rules]) => [state, rules.map(([pattern, emitter, mutator]) => ({pattern, emitter, mutator}))]));
    // like the "include" and "combined" mutators of chroma, which are applied when the lexer is compiled
    const resolveState = (state: string, seen: Set<string>): Array<{pattern: string; emitter: Emitter; mutator: Mutator}> => {
      const result = [];
      for (const rule of raw.get(state) ?? []) {
        const {mutator} = rule;
        if (mutator && !Array.isArray(mutator) && 'include' in mutator) {
          if (!seen.has(mutator.include)) result.push(...resolveState(mutator.include, new Set([...seen, mutator.include])));
        } else if (mutator && !Array.isArray(mutator) && 'combined' in mutator) {
          const name = `__combined_${mutator.combined.join('__')}`;
          if (!raw.has(name)) raw.set(name, mutator.combined.flatMap((combinedState) => raw.get(combinedState) ?? []));
          result.push({...rule, mutator: {push: [name]}});
        } else {
          result.push(rule);
        }
      }
      return result;
    };
    // the states created for "combined" are added while iterating, and also visited
    for (const state of raw.keys()) {
      this.rules.set(state, resolveState(state, new Set([state])).map(({pattern, emitter, mutator}) => ({re: compilePattern(pattern, file.flags), emitter, mutator})));
    }
  }

  // tokenizes like chroma's LexerState.Iterator, the tokens add up to the text, nested calls share the deadline
  tokenize(input: string, initialState = 'root', deadline?: number): Segment[] {
    let text = input;
    const end = text.length;
    if (this.ensureNL && deadline === undefined && !text.endsWith('\n')) text += '\n';
    deadline ??= performance.now() + tokenizeTimeBudget;
    const out: Segment[] = [];
    const stack = [initialState];
    let pos = 0;
    let zeroLengthMatches = 0;
    for (let iteration = 0; pos < end && stack.length; iteration++) {
      if (iteration % 1024 === 0 && performance.now() > deadline) break;
      const state = stack.at(-1)!;
      let match: RegExpExecArray | null = null;
      let rule: Rule | null = null;
      for (const candidate of this.rules.get(state) ?? []) {
        if (!candidate.re) continue;
        candidate.re.lastIndex = pos;
        match = candidate.re.exec(text);
        if (match) {
          rule = candidate;
          break;
        }
      }
      if (!match || !rule || (match[0].length === 0 && ++zeroLengthMatches > maxZeroLengthMatches)) {
        zeroLengthMatches = 0;
        if (text[pos] === '\n' && state !== initialState) {
          stack.splice(0, stack.length, initialState);
          continue;
        }
        out.push([text[pos], '']);
        pos++;
        continue;
      }
      if (match[0].length) zeroLengthMatches = 0;
      pos += match[0].length;
      this.mutate(rule.mutator, stack, state);
      const groups = Array.from(match, (group) => group ?? '');
      this.emit(rule.emitter, groups, out, deadline);
    }
    if (pos < end) out.push([text.slice(pos, end), '']);
    // the token with the added newline is cut
    let length = 0;
    const result: Segment[] = [];
    for (const token of out) {
      if (length >= end) break;
      result.push(length + token[0].length > end ? [token[0].slice(0, end - length), token[1]] : token);
      length += token[0].length;
    }
    return result;
  }

  mutate(mutator: Mutator, stack: string[], state: string) {
    if (!mutator) return;
    if (Array.isArray(mutator)) {
      for (const item of mutator) this.mutate(item, stack, state);
    } else if ('push' in mutator) {
      if (!mutator.push?.length) stack.push(state);
      for (const pushed of mutator.push ?? []) {
        if (pushed === '#pop') stack.pop();
        else stack.push(pushed);
      }
    } else if ('pop' in mutator) {
      stack.splice(Math.max(0, stack.length - mutator.pop));
    }
  }

  emit(emitter: Emitter, groups: string[], out: Segment[], deadline: number) {
    const text = groups[0];
    if (emitter === null || typeof emitter === 'string') {
      if (text) out.push([text, emitter ?? '']);
      return;
    }
    if ('g' in emitter) {
      // like chroma, a wrong number of groups emits the whole match, groups without an emitter are kept as plain text
      if (emitter.g.length !== groups.length - 1) {
        out.push([text, '']);
        return;
      }
      for (const [i, group] of groups.slice(1).entries()) this.emit(emitter.g[i], [group], out, deadline);
      return;
    }
    if ('u' in emitter) {
      const lexer = this.resolve(emitter.u);
      if (lexer) out.push(...lexer.tokenize(text, 'root', deadline));
      else out.push([text, '']);
      return;
    }
    if ('s' in emitter) {
      out.push(...this.tokenize(text, emitter.s, deadline));
      return;
    }
    const [nameGroup, codeGroup, emitters] = emitter.ug;
    const sublexer = this.resolve(groups[nameGroup] ?? '');
    for (const [i, group] of groups.slice(1).entries()) {
      if (i === codeGroup - 1 && sublexer) out.push(...sublexer.tokenize(group, 'root', deadline));
      else this.emit(emitters[i] ?? null, [group], out, deadline);
    }
  }
}

let indexPromise: Promise<LexerEntry[]> | null = null;
const lexers = new Map<string, Promise<ChromaLexer | null>>();
const loadedLexers = new Map<string, ChromaLexer>(); // by lower-case name and aliases

function loadIndex(): Promise<LexerEntry[]> {
  indexPromise ??= (async () => {
    const index: IndexEntry[] = (await import('../../../../assets/chroma-lexers/index.json')).default;
    return index.map((entry) => {
      const patterns = (entry.filenames ?? []).map((glob) => globCompile(glob).regexp);
      return {...entry, patterns, patternsIgnoreCase: patterns.map((re) => new RegExp(re.source, 'i'))};
    });
  })();
  return indexPromise;
}

function findEntry(index: LexerEntry[], name: string): LexerEntry | undefined {
  const lower = name.toLowerCase();
  return index.find((entry) => entry.name.toLowerCase() === lower || entry.aliases?.some((alias) => alias.toLowerCase() === lower));
}

async function loadLexer(index: LexerEntry[], entry: LexerEntry): Promise<ChromaLexer | null> {
  let promise = lexers.get(entry.name);
  if (!promise) {
    promise = (async () => {
      const load = lexerFiles[`../../../../assets/chroma-lexers/${entry.file}.json`];
      if (!load) return null;
      const file = await load();
      // the lexers it delegates to have to be ready, because tokenizing is synchronous
      const usedEntries = (file.using ?? []).flatMap((name) => {
        const used = findEntry(index, name);
        return used && used.name !== entry.name ? [used] : [];
      });
      await Promise.all(usedEntries.map((used) => loadLexer(index, used)));
      const lexer = new ChromaLexer(file, (name) => loadedLexers.get(name.toLowerCase()) ?? null);
      for (const name of [entry.name, ...entry.aliases ?? []]) loadedLexers.set(name.toLowerCase(), lexer);
      return lexer;
    })();
    lexers.set(entry.name, promise);
  }
  return promise;
}

// finds the lexer by the language name of the "linguist-language" attribute, or by the file name like chroma's registry
export async function loadChromaLexer(basename: string, languageName: string): Promise<ChromaLexer | null> {
  const index = await loadIndex();
  let entry = languageName ? findEntry(index, languageName) : undefined;
  entry ??= index.find((candidate) => candidate.patterns.some((re) => re.test(basename)));
  entry ??= index.find((candidate) => candidate.patternsIgnoreCase.some((re) => re.test(basename)));
  return entry ? loadLexer(index, entry) : null;
}
