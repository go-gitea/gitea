import {registerGlobalInitFunc} from '../observer.ts';
import {trString} from '../i18n.ts';
import {parseHunks} from './parse.ts';
import {loadHighlighter} from './highlight.ts';
import {detectEscapes, loadEscapeTables} from './escape.ts';
import {escapeButton, highlightHunkLines, linesToHtml, pairChangedLines, renderUnifiedRows, unifiedColgroup} from './render.ts';

// the "data-config" of RenderDiffSnippet in modules/templates/util_render.go
type SnippetConfig = {
  patch: string;
  filename: string;
  disableHighlight: boolean;
  detectAmbiguous: boolean;
  ambiguousRunesLine: string;
  invisibleRunesLine: string;
  ambiguousCharacter: string;
};

// a read-only unified diff of one file from its patch text, like the excerpts of the PR conversation and the editor's preview
export function initDiffSnippet(): void {
  registerGlobalInitFunc('initDiffSnippet', async (el: HTMLElement) => {
    const config: SnippetConfig = JSON.parse(el.getAttribute('data-config')!);
    const hunks = parseHunks(config.patch);
    const lines = hunks.flatMap((hunk) => hunk.lines);
    const [highlighter, escapeTables] = await Promise.all([
      config.disableHighlight ? null : loadHighlighter(config.filename),
      config.detectAmbiguous ? loadEscapeTables(document.documentElement.lang) : null,
    ]);
    const escapes = lines.map((line) => (escapeTables ? detectEscapes(line.content, escapeTables, (char, confusable) => trString(config.ambiguousCharacter, char, confusable)) : null));
    const lineHtml = linesToHtml(lines, highlightHunkLines(lines, highlighter), pairChangedLines(hunks), escapes);
    const rows = renderUnifiedRows(hunks, lineHtml, {addCommentLabel: null, expanders: new Map(), escapeButtons: escapes.map((escape) => escapeButton(escape, config))});
    el.classList.add('lezer-highlight');
    if (highlighter?.language) el.setAttribute('data-language', highlighter.language);
    el.innerHTML = `<table class="chroma">${unifiedColgroup}<tbody>${rows}</tbody></table>`;
  });
}
