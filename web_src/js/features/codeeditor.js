import tinycolor from 'tinycolor2';
import {basename, extname, isObject, isDarkTheme} from '../utils.js';
import {onInputDebounce} from '../utils/dom.js';

const languagesByFilename = {};
const languagesByExt = {};

const baseOptions = {
  fontFamily: 'var(--fonts-monospace)',
  fontSize: 14, // https://github.com/microsoft/monaco-editor/issues/2242
  guides: {bracketPairs: false, indentation: false},
  links: false,
  minimap: {enabled: false},
  occurrencesHighlight: false,
  overviewRulerLanes: 0,
  renderLineHighlight: 'all',
  renderLineHighlightOnlyWhenFocus: true,
  rulers: false,
  scrollbar: {horizontalScrollbarSize: 6, verticalScrollbarSize: 6},
  scrollBeyondLastLine: false,
  automaticLayout: true,
};

function getEditorconfig(input) {
  try {
    return JSON.parse(input.getAttribute('data-editorconfig'));
  } catch {
    return null;
  }
}

function initLanguages(monaco) {
  for (const {filenames, extensions, id} of monaco.languages.getLanguages()) {
    for (const filename of filenames || []) {
      languagesByFilename[filename] = id;
    }
    for (const extension of extensions || []) {
      languagesByExt[extension] = id;
    }
  }
}

function getLanguage(filename) {
  return languagesByFilename[filename] || languagesByExt[extname(filename)] || 'plaintext';
}

function updateEditor(monaco, editor, filename, lineWrapExts) {
  editor.updateOptions(getFileBasedOptions(filename, lineWrapExts));
  const model = editor.getModel();
  const language = model.getLanguageId();
  const newLanguage = getLanguage(filename);
  if (language !== newLanguage) monaco.editor.setModelLanguage(model, newLanguage);
}

// export editor for customization - https://github.com/go-gitea/gitea/issues/10409
function exportEditor(editor) {
  if (!window.codeEditors) window.codeEditors = [];
  if (!window.codeEditors.includes(editor)) window.codeEditors.push(editor);
}

export async function createMonaco(textarea, filename, editorOpts) {
  const monaco = await import(/* webpackChunkName: "monaco" */'monaco-editor');

  initLanguages(monaco);
  let {language, eol, ...other} = editorOpts;
  if (!language) language = getLanguage(filename);

  const container = document.createElement('div');
  container.className = 'monaco-editor-container';
  textarea.parentNode.append(container);

  // https://github.com/microsoft/monaco-editor/issues/2427
  // also, monaco can only parse 6-digit hex colors, so we convert the colors to that format
  const styles = window.getComputedStyle(document.documentElement);
  const getColor = (name) => tinycolor(styles.getPropertyValue(name).trim()).toString('hex6');

  monaco.editor.defineTheme('gitea', {
    base: isDarkTheme() ? 'vs-dark' : 'vs',
    inherit: true,
    rules: [
      {
        background: getColor('--color-code-bg'),
      }
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
      'scrollbar.shadow': getColor('--color-shadow'),
      'progressBar.background': getColor('--color-primary'),
    }
  });

  // Quick fix: https://github.com/microsoft/monaco-editor/issues/2962
  monaco.languages.register({id: 'vs.editor.nullLanguage'});
  monaco.languages.setLanguageConfiguration('vs.editor.nullLanguage', {});

  // We encode the initial value in JSON on the backend to prevent browsers from
  // discarding the \r during HTML parsing:
  // https://html.spec.whatwg.org/multipage/parsing.html#preprocessing-the-input-stream
  const value = JSON.parse(textarea.getAttribute('data-initial-value') || '""');
  textarea.value = value;
  textarea.removeAttribute('data-initial-value');

  const editor = monaco.editor.create(container, {
    value,
    theme: 'gitea',
    language,
    ...other,
  });

  const model = editor.getModel();

  // Monaco performs auto-detection of dominant EOL in the file, biased towards LF for
  // empty files. If there is an editorconfig value, override this detected value.
  if (eol in monaco.editor.EndOfLineSequence) {
    model.setEOL(monaco.editor.EndOfLineSequence[eol]);
  }

  model.onDidChangeContent(() => {
    textarea.value = editor.getValue();
    textarea.dispatchEvent(new Event('change')); // seems to be needed for jquery-are-you-sure
  });

  exportEditor(editor);

  const loading = document.querySelector('.editor-loading');
  if (loading) loading.remove();

  return {monaco, editor};
}

function getFileBasedOptions(filename, lineWrapExts) {
  return {
    wordWrap: (lineWrapExts || []).includes(extname(filename)) ? 'on' : 'off',
  };
}

function togglePreviewDisplay(previewable) {
  const previewTab = document.querySelector('a[data-tab="preview"]');
  if (!previewTab) return;

  if (previewable) {
    const newUrl = (previewTab.getAttribute('data-url') || '').replace(/(.*)\/.*/, `$1/markup`);
    previewTab.setAttribute('data-url', newUrl);
    previewTab.style.display = '';
  } else {
    previewTab.style.display = 'none';
    // If the "preview" tab was active, user changes the filename to a non-previewable one,
    // then the "preview" tab becomes inactive (hidden), so the "write" tab should become active
    if (previewTab.classList.contains('active')) {
      const writeTab = document.querySelector('a[data-tab="write"]');
      writeTab.click();
    }
  }
}

export async function createCodeEditor(textarea, filenameInput) {
  const filename = basename(filenameInput.value);
  const previewableExts = new Set((textarea.getAttribute('data-previewable-extensions') || '').split(','));
  const lineWrapExts = (textarea.getAttribute('data-line-wrap-extensions') || '').split(',');
  const previewable = previewableExts.has(extname(filename));
  const editorConfig = getEditorconfig(filenameInput);

  togglePreviewDisplay(previewable);

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

function getEditorConfigOptions(ec) {
  if (!isObject(ec)) return {};

  const opts = {};
  opts.detectIndentation = !('indent_style' in ec) || !('indent_size' in ec);
  if ('indent_size' in ec) opts.indentSize = Number(ec.indent_size);
  if ('tab_width' in ec) opts.tabSize = Number(ec.tab_width) || opts.indentSize;
  if ('max_line_length' in ec) opts.rulers = [Number(ec.max_line_length)];
  opts.trimAutoWhitespace = ec.trim_trailing_whitespace === true;
  opts.insertSpaces = ec.indent_style === 'space';
  opts.useTabStops = ec.indent_style === 'tab';
  opts.eol = ec.end_of_line?.toUpperCase();
  return opts;
}
