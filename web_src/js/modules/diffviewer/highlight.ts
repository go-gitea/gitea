import {createHighlighterCore, type HighlighterCore, type ThemeRegistrationRaw} from '@shikijs/core';
import {createJavaScriptRawEngine} from '@shikijs/engine-javascript/raw';
import {shikiLanguages} from './shiki-languages.ts';

// a part of a line with its token classes, empty for unhighlighted text
export type Segment = [text: string, classes: string];

// highlights lines as one document
export type LineHighlighter = (lines: string[]) => Segment[][];

const maxHighlightLength = 512 * 1024;
const maxLineLength = 2000; // longer lines, like minified code, are left plain

// the classes of highlight.css by the TextMate scopes they are given, the deepest matching scope of a token wins
const scopeClasses: Array<[cls: string, scopes: string[]]> = [
  ['tok-comment', ['comment', 'punctuation.definition.comment', 'markup.quote']],
  ['tok-string', ['string', 'punctuation.definition.string', 'markup.raw', 'markup.inline.raw']],
  ['tok-string2', ['string.regexp']],
  ['tok-escape', ['constant.character.escape', 'constant.character.entity']],
  ['tok-number', ['constant.numeric']],
  ['tok-atom', ['constant.language', 'variable.language', 'support.constant']],
  ['tok-bool', ['constant.language.boolean', 'constant.language.null', 'constant.language.nil', 'constant.language.undefined']],
  ['tok-keyword', ['keyword', 'storage.type', 'storage.modifier']],
  ['tok-control', ['keyword.control']],
  ['tok-operator', ['keyword.operator']],
  ['tok-punctuation', ['punctuation', 'meta.brace']],
  ['tok-variableName', ['variable', 'entity.name.variable']],
  ['tok-propertyName', ['variable.other.property', 'variable.other.object.property', 'variable.other.member', 'support.variable.property', 'meta.object-literal.key', 'support.type.property-name']],
  ['tok-tag', ['entity.name.tag', 'punctuation.definition.tag', 'support.type.property-name.json']],
  ['tok-attribute', ['entity.other.attribute-name']],
  ['tok-typeName', ['entity.name.type', 'support.type', 'support.class', 'entity.other.inherited-class', 'storage.type.primitive', 'storage.type.built-in', 'storage.type.numeric', 'storage.type.generic', 'keyword.type']],
  ['tok-className', ['entity.name.class', 'entity.name.type.class', 'entity.name.struct', 'entity.name.type.struct']],
  ['tok-namespace', ['entity.name.namespace', 'entity.name.module', 'entity.name.type.module', 'entity.name.type.package', 'entity.name.package', 'support.module']],
  ['tok-function', ['entity.name.function', 'support.function']],
  ['tok-macroName', ['entity.name.function.preprocessor', 'entity.name.function.macro', 'entity.name.macro']],
  ['tok-labelName', ['entity.name.label']],
  ['tok-meta', ['meta.preprocessor', 'keyword.control.directive', 'keyword.other.preprocessor', 'punctuation.definition.directive']],
  ['tok-decorator', ['meta.decorator', 'entity.name.function.decorator', 'punctuation.decorator', 'meta.annotation', 'storage.type.annotation', 'entity.name.type.annotation']],
  ['tok-invalid', ['invalid']],
  ['tok-link', ['markup.underline.link', 'string.other.link', 'constant.other.reference.link']],
  ['tok-heading', ['markup.heading', 'entity.name.section']],
  ['tok-emphasis', ['markup.italic']],
  ['tok-strong', ['markup.bold']],
  ['tok-inserted', ['markup.inserted']],
  ['tok-deleted', ['markup.deleted']],
];

// linguist names which neither match an id, a name nor an alias of a shiki grammar
const linguistOverrides: Record<string, string> = {
  'apacheconf': 'apache',
  'batchfile': 'bat',
  'cuda': 'cpp',
  'editorconfig': 'ini',
  'fortran': 'fortran-fixed-form',
  'gettext catalog': 'po',
  'git config': 'ini',
  'gradle': 'groovy',
  'html+django': 'jinja',
  'html+erb': 'erb',
  'html+php': 'php',
  'html+razor': 'razor',
  'java properties': 'ini',
  'jsonl': 'json',
  'jupyter notebook': 'json',
  'justfile': 'just',
  'lean 4': 'lean',
  'mathematica': 'wolfram',
  'plpgsql': 'sql',
  'protocol buffer': 'proto',
  'sqlpl': 'sql',
  'starlark': 'python',
  'svg': 'xml',
  'tsql': 'sql',
  'unix assembly': 'asm',
  'visual basic .net': 'vb',
};

// themes only take colors, so every class gets a stand-in color which the tokens are mapped back from
const themeName = 'gitea';
const classByColor = new Map<string, string>();
const theme: ThemeRegistrationRaw = {
  name: themeName,
  settings: [{settings: {foreground: '#000000'}}, ...scopeClasses.map(([cls, scope], idx) => {
    const color = `#${(idx + 1).toString(16).padStart(6, '0')}`;
    classByColor.set(color, cls);
    return {scope, settings: {foreground: color}};
  })],
};

let corePromise: Promise<HighlighterCore> | null = null;
const highlighters = new Map<string, Promise<LineHighlighter | null>>();

// the id of the shiki grammar for a language name of linguist or of the "linguist-language" attribute
export function findLanguage(language: string): string | null {
  const lower = language.toLowerCase();
  const id = linguistOverrides[lower] ?? lower.replaceAll(' ', '-');
  if (id in shikiLanguages) return id;
  for (const [key, lang] of Object.entries(shikiLanguages)) {
    if (lang.name.toLowerCase() === lower || lang.aliases.includes(lower)) return key;
  }
  return null;
}

// highlighting is skipped for very long content, which would block the page
export function highlightLines(highlighter: LineHighlighter | null | undefined, lines: string[]): Segment[][] {
  if (!highlighter || lines.reduce((length, line) => length + line.length + 1, 0) > maxHighlightLength) {
    return lines.map((line) => [[line, '']]);
  }
  return highlighter(lines);
}

async function createHighlighter(language: string): Promise<LineHighlighter | null> {
  const id = findLanguage(language);
  if (!id) return null;
  corePromise ??= createHighlighterCore({engine: createJavaScriptRawEngine(), themes: [theme]});
  const core = await corePromise;
  await core.loadLanguage(shikiLanguages[id].load);
  return (lines) => {
    const tokens = core.codeToTokensBase(lines.join('\n'), {lang: id, theme: themeName, includeExplanation: false, tokenizeMaxLineLength: maxLineLength});
    return tokens.map((line) => line.flatMap((token): Segment[] => (token.content ? [[token.content, classByColor.get(token.color?.toLowerCase() ?? '') ?? '']] : [])));
  };
}

async function createHighlighterSafe(language: string): Promise<LineHighlighter | null> {
  try {
    return await createHighlighter(language);
  } catch (error) {
    console.error(`Failed to load the highlighter for ${language}`, error);
    return null;
  }
}

// starts loading as early as possible, so files are usually highlighted on their first render
export function loadHighlighter(language: string | undefined): Promise<LineHighlighter | null> {
  if (!language) return Promise.resolve(null);
  const key = language.toLowerCase();
  let promise = highlighters.get(key);
  if (!promise) {
    promise = createHighlighterSafe(language);
    highlighters.set(key, promise);
  }
  return promise;
}
