import tinycolor from 'tinycolor2';
import {basename, extname, isObject, isDarkTheme} from '../utils.ts';
import {onInputDebounce} from '../utils/dom.ts';
import type MonacoNamespace from 'monaco-editor';

type Monaco = typeof MonacoNamespace;
type IStandaloneCodeEditor = MonacoNamespace.editor.IStandaloneCodeEditor;
type IEditorOptions = MonacoNamespace.editor.IEditorOptions;
type IGlobalEditorOptions = MonacoNamespace.editor.IGlobalEditorOptions;
type ITextModelUpdateOptions = MonacoNamespace.editor.ITextModelUpdateOptions;
type MonacoOpts = IEditorOptions & IGlobalEditorOptions & ITextModelUpdateOptions;

type EditorConfig = {
  indent_style?: 'tab' | 'space',
  indent_size?: string | number, // backend emits this as string
  tab_width?: string | number, // backend emits this as string
  end_of_line?: 'lf' | 'cr' | 'crlf',
  charset?: 'latin1' | 'utf-8' | 'utf-8-bom' | 'utf-16be' | 'utf-16le',
  trim_trailing_whitespace?: boolean,
  insert_final_newline?: boolean,
  root?: boolean,
}

const languagesByFilename: Record<string, string> = {};
const languagesByExt: Record<string, string> = {};

const baseOptions: MonacoOpts = {
  fontFamily: 'var(--fonts-monospace)',
  fontSize: 14, // https://github.com/microsoft/monaco-editor/issues/2242
  guides: {bracketPairs: false, indentation: false},
  links: false,
  minimap: {enabled: false},
  occurrencesHighlight: 'off',
  overviewRulerLanes: 0,
  renderLineHighlight: 'all',
  renderLineHighlightOnlyWhenFocus: true,
  rulers: [],
  scrollbar: {horizontalScrollbarSize: 6, verticalScrollbarSize: 6},
  scrollBeyondLastLine: false,
  automaticLayout: true,
};

function getEditorconfig(input: HTMLInputElement): EditorConfig | null {
  const json = input.getAttribute('data-editorconfig');
  if (!json) return null;
  try {
    return JSON.parse(json);
  } catch {
    return null;
  }
}

function initLanguages(monaco: Monaco): void {
  for (const {filenames, extensions, id} of monaco.languages.getLanguages()) {
    for (const filename of filenames || []) {
      languagesByFilename[filename] = id;
    }
    for (const extension of extensions || []) {
      languagesByExt[extension] = id;
    }
    if (id === 'typescript') {
      monaco.languages.typescript.typescriptDefaults.setCompilerOptions({
        // this is needed to suppress error annotations in tsx regarding missing --jsx flag.
        jsx: monaco.languages.typescript.JsxEmit.Preserve,
      });
    }
  }
}

function getLanguage(filename: string): string {
  return languagesByFilename[filename] || languagesByExt[extname(filename)] || 'plaintext';
}

function updateEditor(monaco: Monaco, editor: IStandaloneCodeEditor, filename: string, lineWrapExts: string[]): void {
  editor.updateOptions(getFileBasedOptions(filename, lineWrapExts));
  const model = editor.getModel();
  if (!model) return;
  const language = model.getLanguageId();
  const newLanguage = getLanguage(filename);
  if (language !== newLanguage) monaco.editor.setModelLanguage(model, newLanguage);
  // TODO: Need to update the model uri with the new filename, but there is no easy way currently, see
  // https://github.com/microsoft/monaco-editor/discussions/3751
}

// export editor for customization - https://github.com/go-gitea/gitea/issues/10409
function exportEditor(editor: IStandaloneCodeEditor): void {
  if (!window.codeEditors) window.codeEditors = [];
  if (!window.codeEditors.includes(editor)) window.codeEditors.push(editor);
}

function updateTheme(monaco: Monaco): void {
  // https://github.com/microsoft/monaco-editor/issues/2427
  // also, monaco can only parse 6-digit hex colors, so we convert the colors to that format
  const styles = window.getComputedStyle(document.documentElement);
  const getColor = (name: string) => tinycolor(styles.getPropertyValue(name).trim()).toString('hex6');

  monaco.editor.defineTheme('gitea', {
    base: isDarkTheme() ? 'vs-dark' : 'vs',
    inherit: true,
    rules: [
      {
        background: getColor('--color-code-bg'),
        token: '',
      },
    ],
    colors: {
      'editor.background': getColor('--color-code-bg'),
      'editor.foreground': getColor('--color-text'),
      'editor.inactiveSelectionBackground': getColor('--color-primary-light-4'),
      'editor.lineHighlightBackground': getColor('--color-editor-line-highlight'),
      'editor.selectionBackground': getColor('--color-primary-light-3'),
      'editor.selectionForeground': getColor('--color-primary-light-3'),
      'editorLineNumber.background': getColor('--color-code-bg'),
      'editorLineNumber.foreground': getColor('--color-secondary-dark-6'),
      'editorWidget.background': getColor('--color-body'),
      'editorWidget.border': getColor('--color-secondary'),
      'input.background': getColor('--color-input-background'),
      'input.border': getColor('--color-input-border'),
      'input.foreground': getColor('--color-input-text'),
      'scrollbar.shadow': getColor('--color-shadow-opaque'),
      'progressBar.background': getColor('--color-primary'),
      'focusBorder': '#0000', // prevent blue border
    },
  });
}

type CreateMonacoOpts = MonacoOpts & {language?: string};

export async function createMonaco(textarea: HTMLTextAreaElement, filename: string, opts: CreateMonacoOpts): Promise<{monaco: Monaco, editor: IStandaloneCodeEditor}> {
  const monaco = await import(/* webpackChunkName: "monaco" */'monaco-editor');

  initLanguages(monaco);
  let {language, ...other} = opts;
  if (!language) language = getLanguage(filename);

  const container = document.createElement('div');
  container.className = 'monaco-editor-container';
  if (!textarea.parentNode) throw new Error('Parent node absent');
  textarea.parentNode.append(container);

  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    updateTheme(monaco);
  });
  updateTheme(monaco);

  const model = monaco.editor.createModel(textarea.value, language, monaco.Uri.file(filename));

  const editor = monaco.editor.create(container, {
    model,
    theme: 'gitea',
    ...other,
  });

  monaco.editor.addKeybindingRules([
    {keybinding: monaco.KeyCode.Enter, command: null}, // disable enter from accepting code completion
  ]);

  model.onDidChangeContent(() => {
    textarea.value = editor.getValue({
      preserveBOM: true,
      lineEnding: '',
    });
    textarea.dispatchEvent(new Event('change')); // seems to be needed for jquery-are-you-sure
  });

  exportEditor(editor);

  const loading = document.querySelector('.editor-loading');
  if (loading) loading.remove();

  return {monaco, editor};
}

function getFileBasedOptions(filename: string, lineWrapExts: string[]): MonacoOpts {
  return {
    wordWrap: (lineWrapExts || []).includes(extname(filename)) ? 'on' : 'off',
  };
}

function togglePreviewDisplay(previewable: boolean): void {
  const previewTab = document.querySelector<HTMLElement>('a[data-tab="preview"]');
  if (!previewTab) return;

  if (previewable) {
    previewTab.style.display = '';
  } else {
    previewTab.style.display = 'none';
    // If the "preview" tab was active, user changes the filename to a non-previewable one,
    // then the "preview" tab becomes inactive (hidden), so the "write" tab should become active
    if (previewTab.classList.contains('active')) {
      const writeTab = document.querySelector<HTMLElement>('a[data-tab="write"]');
      writeTab?.click();
    }
  }
}

export async function createCodeEditor(textarea: HTMLTextAreaElement, filenameInput: HTMLInputElement): Promise<IStandaloneCodeEditor> {
  const filename = basename(filenameInput.value);
  const previewableExts = new Set((textarea.getAttribute('data-previewable-extensions') || '').split(','));
  const lineWrapExts = (textarea.getAttribute('data-line-wrap-extensions') || '').split(',');
  const isPreviewable = previewableExts.has(extname(filename));
  const editorConfig = getEditorconfig(filenameInput);

  togglePreviewDisplay(isPreviewable);

  const {monaco, editor} = await createMonaco(textarea, filename, {
    ...baseOptions,
    ...getFileBasedOptions(filenameInput.value, lineWrapExts),
    ...getEditorConfigOptions(editorConfig),
  });

  filenameInput.addEventListener('input', onInputDebounce(() => {
    const filename = filenameInput.value;
    const previewable = previewableExts.has(extname(filename));
    togglePreviewDisplay(previewable);
    updateEditor(monaco, editor, filename, lineWrapExts);
  }));

  return editor;
}

function getEditorConfigOptions(ec: EditorConfig | null): MonacoOpts {
  if (!ec || !isObject(ec)) return {};

  const opts: MonacoOpts = {};
  opts.detectIndentation = !('indent_style' in ec) || !('indent_size' in ec);

  if ('indent_size' in ec) {
    opts.indentSize = Number(ec.indent_size);
  }
  if ('tab_width' in ec) {
    opts.tabSize = Number(ec.tab_width) || Number(ec.indent_size);
  }
  if ('max_line_length' in ec) {
    opts.rulers = [Number(ec.max_line_length)];
  }

  opts.trimAutoWhitespace = ec.trim_trailing_whitespace === true;
  opts.insertSpaces = ec.indent_style === 'space';
  opts.useTabStops = ec.indent_style === 'tab';
  return opts;
}
