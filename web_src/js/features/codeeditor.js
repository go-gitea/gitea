import {basename, extname, isObject, isDarkTheme} from '../utils.js';

const languagesByFilename = {};
const languagesByExt = {};

function getEditorconfig(input) {
  try {
    return JSON.parse(input.dataset.editorconfig);
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

function updateEditor(monaco, editor, filenameInput) {
  const newFilename = filenameInput.value;
  editor.updateOptions(getOptions(filenameInput));
  const model = editor.getModel();
  const language = model.getModeId();
  const newLanguage = getLanguage(newFilename);
  if (language !== newLanguage) monaco.editor.setModelLanguage(model, newLanguage);
}

// export editor for customization - https://github.com/go-gitea/gitea/issues/10409
function exportEditor(editor) {
  if (!window.codeEditors) window.codeEditors = [];
  if (!window.codeEditors.includes(editor)) window.codeEditors.push(editor);
}

export async function createCodeEditor(textarea, filenameInput, previewFileModes) {
  const filename = basename(filenameInput.value);
  const previewLink = document.querySelector('a[data-tab=preview]');
  const markdownExts = (textarea.dataset.markdownFileExts || '').split(',');
  const lineWrapExts = (textarea.dataset.lineWrapExtensions || '').split(',');
  const isMarkdown = markdownExts.includes(extname(filename));

  if (previewLink) {
    if (isMarkdown && (previewFileModes || []).includes('markdown')) {
      previewLink.dataset.url = previewLink.dataset.url.replace(/(.*)\/.*/i, `$1/markdown`);
      previewLink.style.display = '';
    } else {
      previewLink.style.display = 'none';
    }
  }

  const monaco = await import(/* webpackChunkName: "monaco" */'monaco-editor');
  initLanguages(monaco);

  const container = document.createElement('div');
  container.className = 'monaco-editor-container';
  textarea.parentNode.appendChild(container);

  const editor = monaco.editor.create(container, {
    value: textarea.value,
    language: getLanguage(filename),
    ...getOptions(filenameInput, lineWrapExts),
  });

  const model = editor.getModel();
  model.onDidChangeContent(() => {
    textarea.value = editor.getValue();
    textarea.dispatchEvent(new Event('change')); // seems to be needed for jquery-are-you-sure
  });

  window.addEventListener('resize', () => {
    editor.layout();
  });

  filenameInput.addEventListener('keyup', () => {
    updateEditor(monaco, editor, filenameInput);
  });

  const loading = document.querySelector('.editor-loading');
  if (loading) loading.remove();

  exportEditor(editor);

  return editor;
}

function getOptions(filenameInput, lineWrapExts) {
  const ec = getEditorconfig(filenameInput);
  const theme = isDarkTheme() ? 'vs-dark' : 'vs';
  const wordWrap = (lineWrapExts || []).includes(extname(filenameInput.value)) ? 'on' : 'off';

  const opts = {theme, wordWrap};
  if (isObject(ec)) {
    opts.detectIndentation = !('indent_style' in ec) || !('indent_size' in ec);
    if ('indent_size' in ec) opts.indentSize = Number(ec.indent_size);
    if ('tab_width' in ec) opts.tabSize = Number(ec.tab_width) || opts.indentSize;
    if ('max_line_length' in ec) opts.rulers = [Number(ec.max_line_length)];
    opts.trimAutoWhitespace = ec.trim_trailing_whitespace === true;
    opts.insertSpaces = ec.indent_style === 'space';
    opts.useTabStops = ec.indent_style === 'tab';
  }

  return opts;
}
