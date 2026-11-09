import {registerGlobalInitFunc} from '../observer.ts';
import {html, htmlRaw} from '../../utils/html.ts';
import {trString} from '../i18n.ts';
import {getCurrentLocale} from '../../utils.ts';
import {parseHunks} from './parse.ts';
import {loadHighlighter} from './highlight.ts';
import {detectEscapes, loadEscapeTables} from './escape.ts';
import {escapeButton, highlightHunkLines, linesToHtml, pairChangedLines, renderUnifiedRows, unifiedColgroup} from './render.ts';

// the "data-config" of RenderDiffSnippet in modules/templates/util_render.go
type SnippetConfig = {
  patch: string;
  language: string;
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
      loadHighlighter(config.language),
      config.detectAmbiguous ? loadEscapeTables(getCurrentLocale()) : null,
    ]);
    const escapes = lines.map((line) => (escapeTables ? detectEscapes(line.content, escapeTables, (char, confusable) => trString(config.ambiguousCharacter, char, confusable)) : null));
    const lineHtml = linesToHtml(lines, highlightHunkLines(lines, highlighter), pairChangedLines(hunks), escapes);
    const rows = renderUnifiedRows(hunks, lineHtml, {addCommentLabel: null, expanders: new Map(), escapeButtons: escapes.map((escape) => escapeButton(escape, config))});
    el.classList.add('code-highlight');
    el.innerHTML = html`<table class="chroma" data-paste-markdown-skip>${htmlRaw(unifiedColgroup)}<tbody>${htmlRaw(rows)}</tbody></table>`;
  });
}
