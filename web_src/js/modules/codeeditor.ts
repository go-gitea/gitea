import {basename, extname, isObject} from '../utils.ts';
import {createElementFromHTML, onInputDebounce, toggleElem} from '../utils/dom.ts';
import {html, htmlRaw} from '../utils/html.ts';
import {svg} from '../svg.ts';
import type {LanguageDescription} from '@codemirror/language';
import type {Compartment} from '@codemirror/state';
import type {EditorView, ViewUpdate} from '@codemirror/view';

type CodeEditorConfig = {
  indent_style?: 'tab' | 'space',
  indent_size?: number,
  tab_width?: string | number, // backend emits this as string
  trim_trailing_whitespace?: boolean,
};

type EditorOptions = {
  indentStyle: string;
  indentSize?: number;
  tabSize?: number;
  wordWrap: boolean;
  trimTrailingWhitespace: boolean;
};

export type CodemirrorEditor = {
  view: EditorView;
  trimTrailingWhitespace: boolean;
  languages: LanguageDescription[];
  compartments: {
    wordWrap: Compartment;
    language: Compartment;
    tabSize: Compartment;
    indentUnit: Compartment;
  };
};

function getCodeEditorConfig(input: HTMLInputElement): CodeEditorConfig | null {
  const json = input.getAttribute('data-code-editor-config');
  if (!json) return null;
  try {
    return JSON.parse(json);
  } catch {
    return null;
  }
}

// export editor for customization - https://github.com/go-gitea/gitea/issues/10409
function exportEditor(editor: EditorView): void {
  if (!window.codeEditors) window.codeEditors = [];
  if (!window.codeEditors.includes(editor)) window.codeEditors.push(editor);
}

async function importCodemirror() {
  const [view, state, search, language, commands, autocomplete, languageData, highlight, indentMarkers] = await Promise.all([
    import(/* webpackChunkName: "codemirror" */ '@codemirror/view'),
    import(/* webpackChunkName: "codemirror" */ '@codemirror/state'),
    import(/* webpackChunkName: "codemirror" */ '@codemirror/search'),
    import(/* webpackChunkName: "codemirror" */ '@codemirror/language'),
    import(/* webpackChunkName: "codemirror" */ '@codemirror/commands'),
    import(/* webpackChunkName: "codemirror" */ '@codemirror/autocomplete'),
    import(/* webpackChunkName: "codemirror" */ '@codemirror/language-data'),
    import(/* webpackChunkName: "codemirror" */ '@lezer/highlight'),
    import(/* webpackChunkName: "codemirror" */ '@replit/codemirror-indentation-markers'),
  ]);
  return {view, state, search, language, commands, autocomplete, languageData, highlight, indentMarkers};
}

async function createCodemirrorEditor(
  textarea: HTMLTextAreaElement,
  filename: string,
  editorOpts: EditorOptions,
): Promise<CodemirrorEditor> {
  const cm = await importCodemirror();
  const languageDescriptions = cm.languageData.languages;
  const matchedLang = cm.language.LanguageDescription.matchFilename(languageDescriptions, filename);

  const container = document.createElement('div');
  container.className = 'code-editor-container';
  if (!textarea.parentNode) throw new Error('Parent node absent');
  textarea.parentNode.append(container);

  const wordWrap = new cm.state.Compartment();
  const language = new cm.state.Compartment();
  const tabSize = new cm.state.Compartment();
  const indentUnitComp = new cm.state.Compartment();

  const view = new cm.view.EditorView({
    doc: textarea.value,
    parent: container,
    extensions: [
      cm.view.lineNumbers(),
      cm.language.codeFolding({
        placeholderDOM(_view: EditorView, onclick: (event: Event) => void) {
          const el = createElementFromHTML(html`<span class="cm-foldPlaceholder">${htmlRaw(svg('octicon-kebab-horizontal', 16))}</span>`);
          el.addEventListener('click', onclick);
          return el as unknown as HTMLElement;
        },
      }),
      cm.language.foldGutter({
        markerDOM(open: boolean) {
          return createElementFromHTML(svg(open ? 'octicon-chevron-down' : 'octicon-chevron-right', 16));
        },
      }),
      cm.view.highlightActiveLineGutter(),
      cm.view.highlightSpecialChars(),
      cm.view.highlightActiveLine(),
      cm.view.drawSelection(),
      cm.view.dropCursor(),
      cm.view.rectangularSelection(),
      cm.view.crosshairCursor(),
      textarea.getAttribute('data-placeholder') ? cm.view.placeholder(textarea.getAttribute('data-placeholder')!) : [],
      editorOpts.trimTrailingWhitespace ? cm.view.highlightTrailingWhitespace() : [],
      cm.search.search({top: true}),
      cm.search.highlightSelectionMatches(),
      cm.view.keymap.of([
        ...cm.autocomplete.closeBracketsKeymap,
        ...cm.commands.defaultKeymap,
        ...cm.commands.historyKeymap,
        ...cm.search.searchKeymap,
        ...cm.language.foldKeymap,
        ...cm.autocomplete.completionKeymap,
        cm.commands.indentWithTab,
      ]),
      cm.state.EditorState.allowMultipleSelections.of(true),
      cm.language.indentOnInput(),
      cm.language.syntaxHighlighting(cm.highlight.classHighlighter),
      cm.language.bracketMatching(),
      indentUnitComp.of(
        cm.language.indentUnit.of(
          editorOpts.indentStyle === 'tab' ? '\t' : ' '.repeat(editorOpts.indentSize || 4),
        ),
      ),
      cm.autocomplete.closeBrackets(),
      cm.autocomplete.autocompletion(),
      cm.state.EditorState.languageData.of(() => [{autocomplete: cm.autocomplete.completeAnyWord}]),
      cm.indentMarkers.indentationMarkers({
        colors: {
          light: 'var(--color-secondary)',
          dark: 'var(--color-secondary)',
          activeLight: 'var(--color-secondary-dark-2)',
          activeDark: 'var(--color-secondary-dark-2)',
        },
      }),
      cm.commands.history(),
      tabSize.of(cm.state.EditorState.tabSize.of(editorOpts.tabSize || 4)),
      wordWrap.of(editorOpts.wordWrap ? cm.view.EditorView.lineWrapping : []),
      language.of(matchedLang ? await matchedLang.load() : []),
      cm.view.EditorView.updateListener.of((update: ViewUpdate) => {
        if (update.docChanged) {
          textarea.value = update.state.doc.toString();
          textarea.dispatchEvent(new Event('change')); // needed for jquery-are-you-sure
        }
      }),
    ],
  });

  exportEditor(view);

  const loading = document.querySelector('.editor-loading');
  if (loading) loading.remove();

  return {
    view,
    trimTrailingWhitespace: editorOpts.trimTrailingWhitespace,
    languages: languageDescriptions,
    compartments: {wordWrap, language, tabSize, indentUnit: indentUnitComp},
  };
}

function setupEditorOptionListeners(textarea: HTMLTextAreaElement, editor: CodemirrorEditor): void {
  const elEditorOptions = textarea.closest('form')?.querySelector('.code-editor-options');
  if (!elEditorOptions) return;

  const {compartments, view} = editor;
  const indentStyleSelect = elEditorOptions.querySelector<HTMLSelectElement>('.js-indent-style-select');
  const indentSizeSelect = elEditorOptions.querySelector<HTMLSelectElement>('.js-indent-size-select');

  const applyIndentSettings = async (style: string, size: number) => {
    const cm = await importCodemirror();
    view.dispatch({
      effects: [
        compartments.indentUnit.reconfigure(cm.language.indentUnit.of(style === 'tab' ? '\t' : ' '.repeat(size))),
        compartments.tabSize.reconfigure(cm.state.EditorState.tabSize.of(size)),
      ],
    });
  };

  indentStyleSelect?.addEventListener('change', () => {
    applyIndentSettings(indentStyleSelect.value, Number(indentSizeSelect?.value) || 4);
  });

  indentSizeSelect?.addEventListener('change', () => {
    applyIndentSettings(indentStyleSelect?.value || 'space', Number(indentSizeSelect.value) || 4);
  });

  elEditorOptions.querySelector<HTMLSelectElement>('.js-line-wrap-select')?.addEventListener('change', async (e) => {
    const target = e.target as HTMLSelectElement;
    const cm = await importCodemirror();
    view.dispatch({
      effects: compartments.wordWrap.reconfigure(target.value === 'on' ? cm.view.EditorView.lineWrapping : []),
    });
  });
}

function getFileBasedOptions(filename: string, lineWrapExts: string[]): Pick<EditorOptions, 'wordWrap'> {
  return {
    wordWrap: (lineWrapExts || []).includes(extname(filename)),
  };
}

function togglePreviewDisplay(previewable: boolean): void {
  // FIXME: here and below, the selector is too broad, it should only query in the editor related scope
  const previewTab = document.querySelector<HTMLElement>('a[data-tab="preview"]');
  // the "preview tab" exists for "file code editor", but doesn't exist for "git hook editor"
  if (!previewTab) return;

  toggleElem(previewTab, previewable);
  if (previewable) return;

  // If not previewable but the "preview" tab was active (user changes the filename to a non-previewable one),
  // then the "preview" tab becomes inactive (hidden), so the "write" tab should become active
  if (previewTab.classList.contains('active')) {
    const writeTab = document.querySelector<HTMLElement>('a[data-tab="write"]');
    writeTab?.click(); // TODO: it shouldn't need null-safe operator, writeTab must exist
  }
}

export async function createCodeEditor(textarea: HTMLTextAreaElement, filenameInput?: HTMLInputElement | string): Promise<CodemirrorEditor> {
  const filename = basename(typeof filenameInput === 'string' ? filenameInput : filenameInput?.value || '');
  const previewableExts = new Set((textarea.getAttribute('data-previewable-extensions') || '').split(','));
  const lineWrapExts = (textarea.getAttribute('data-line-wrap-extensions') || '').split(',');

  let editorOpts: EditorOptions;
  if (typeof filenameInput === 'object') {
    const editorConfig = getCodeEditorConfig(filenameInput);
    togglePreviewDisplay(previewableExts.has(extname(filename)));
    const configOpts = getEditorConfigOptions(editorConfig);
    editorOpts = {
      ...getFileBasedOptions(filenameInput.value, lineWrapExts),
      indentStyle: configOpts.indentStyle || 'space',
      trimTrailingWhitespace: false,
      ...configOpts,
    };
  } else {
    editorOpts = {indentStyle: 'tab', tabSize: 4, wordWrap: false, trimTrailingWhitespace: false};
  }

  const editor = await createCodemirrorEditor(textarea, filename, editorOpts);

  if (typeof filenameInput === 'object') {
    filenameInput.addEventListener('input', onInputDebounce(async () => {
      const newFilename = filenameInput.value;
      togglePreviewDisplay(previewableExts.has(extname(newFilename)));
      await updateEditorLanguage(editor, newFilename, lineWrapExts);
    }));
  }

  setupEditorOptionListeners(textarea, editor);

  return editor;
}

async function updateEditorLanguage(editor: CodemirrorEditor, filename: string, lineWrapExts: string[]): Promise<void> {
  const {view: cmView, language: cmLanguage} = await importCodemirror();
  const {compartments, view, languages: editorLanguages} = editor;

  const fileOption = getFileBasedOptions(filename, lineWrapExts);
  const newLanguage = cmLanguage.LanguageDescription.matchFilename(editorLanguages, filename);
  view.dispatch({
    effects: [
      compartments.wordWrap.reconfigure(
        fileOption.wordWrap ? cmView.EditorView.lineWrapping : [],
      ),
      compartments.language.reconfigure(newLanguage ? await newLanguage.load() : []),
    ],
  });
}

export function trimTrailingWhitespaceFromView(view: EditorView): void {
  const changes = [];
  const doc = view.state.doc;
  for (let i = 1; i <= doc.lines; i++) {
    const line = doc.line(i);
    const trimmed = line.text.replace(/\s+$/, '');
    if (trimmed.length < line.text.length) {
      changes.push({from: line.from + trimmed.length, to: line.to});
    }
  }
  if (changes.length) view.dispatch({changes});
}

function getEditorConfigOptions(ec: CodeEditorConfig | null): Partial<EditorOptions> {
  if (!ec || !isObject(ec)) return {indentStyle: 'space'};

  const opts: Partial<EditorOptions> = {
    indentStyle: ec.indent_style || 'space',
    trimTrailingWhitespace: ec.trim_trailing_whitespace === true,
  };
  if (ec.indent_size) opts.indentSize = ec.indent_size;
  if (ec.tab_width) opts.tabSize = Number(ec.tab_width) || opts.indentSize;
  return opts;
}
