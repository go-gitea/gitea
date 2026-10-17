import {buildLanguageDescriptions, importCodemirror} from '../codeeditor/main.ts';
import {basename as fileBasename} from '../../utils.ts';
import {loadChromaLexer} from './chroma.ts';

// a part of a line with its token classes, empty for unhighlighted text
export type Segment = [text: string, classes: string];

// highlights lines as one document, language is the lower-case name of a CodeMirror language for its specific colors
export type LineHighlighter = {language: string; highlight: (lines: string[]) => Segment[][]};

const maxHighlightLength = 512 * 1024;

let languagesPromise: ReturnType<typeof loadLanguages> | null = null;
const highlighters = new Map<string, Promise<LineHighlighter | null>>();

async function loadLanguages() {
  const cm = await importCodemirror();
  return {cm, languages: buildLanguageDescriptions(cm)};
}

// highlighting is skipped for very long content, which would block the page
export function highlightLines(highlighter: LineHighlighter | null | undefined, lines: string[]): Segment[][] {
  if (!highlighter || lines.reduce((length, line) => length + line.length + 1, 0) > maxHighlightLength) {
    return lines.map((line) => [[line, '']]);
  }
  return highlighter.highlight(lines);
}

function tokensToLines(tokens: Segment[]): Segment[][] {
  const result: Segment[][] = [[]];
  for (const [text, classes] of tokens) {
    const parts = text.split('\n');
    for (const [idx, part] of parts.entries()) {
      if (idx > 0) result.push([]);
      if (part) result.at(-1)!.push([part, classes]);
    }
  }
  return result;
}

async function createHighlighter(basename: string, languageName: string): Promise<LineHighlighter | null> {
  languagesPromise ??= loadLanguages();
  const {cm, languages} = await languagesPromise;
  const {LanguageDescription} = cm.language;
  const description = (languageName && LanguageDescription.matchLanguageName(languages, languageName, false)) ||
    LanguageDescription.matchFilename(languages, basename);
  if (!description) {
    // the languages without a CodeMirror grammar are highlighted by chroma's lexers, their classes are styled by chroma.css
    const lexer = await loadChromaLexer(basename, languageName);
    return lexer && {language: '', highlight: (lines) => tokensToLines(lexer.tokenize(lines.join('\n')))};
  }
  const {language} = await description.load();
  return {language: description.name.toLowerCase(), highlight: (lines) => {
    const code = lines.join('\n');
    const result: Segment[][] = [];
    let current: Segment[] = [];
    cm.highlight.highlightCode(code, language.parser.parse(code), cm.highlight.classHighlighter, (text, classes) => {
      current.push([text, classes]);
    }, () => {
      result.push(current);
      current = [];
    });
    result.push(current);
    return result;
  }};
}

async function createHighlighterSafe(basename: string, languageName: string): Promise<LineHighlighter | null> {
  try {
    return await createHighlighter(basename, languageName);
  } catch (error) {
    console.error(`Failed to load the highlighter for ${basename}`, error);
    return null;
  }
}

// starts loading as early as possible, so files are usually highlighted on their first render,
// languageName comes from the "linguist-language" attribute and takes precedence over the file name
export function loadHighlighter(filename: string, languageName = ''): Promise<LineHighlighter | null> {
  const basename = fileBasename(filename);
  // not by extension, some languages match whole file names like "CMakeLists.txt" or "Dockerfile.prod"
  const key = languageName ? `lang:${languageName.toLowerCase()}` : basename;
  let promise = highlighters.get(key);
  if (!promise) {
    promise = createHighlighterSafe(basename, languageName);
    highlighters.set(key, promise);
  }
  return promise;
}
