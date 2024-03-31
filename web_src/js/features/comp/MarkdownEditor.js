import '@github/markdown-toolbar-element';
import '@github/text-expander-element';
import $ from 'jquery';
import {autosize} from '../../utils/dom.js';
import {initTextareaPaste} from './Paste.js';
import {renderPreviewPanelContent} from '../repo-editor.js';
import {initTextExpander} from './TextExpander.js';
import {POST} from '../../modules/fetch.js';

let elementIdCounter = 0;

/**
 * validate if the given textarea is non-empty.
 * @param {HTMLElement} textarea - The textarea element to be validated.
 * @returns {boolean} returns true if validation succeeded.
 */
export function validateTextareaNonEmpty(textarea) {
  if (!textarea.value) {
    textarea.required = true;
    const form = textarea.closest('form');
    form?.reportValidity();
    return false;
  }
  return true;
}

class MarkdownEditor {
  constructor(container, options = {}) {
    container._giteaMarkdownEditor = this;
    this.options = options;
    this.container = container;
  }

  async init() {
    this.setupContainer();
    this.setupTab();
    this.setupDropzone();
    this.setupTextarea();
  }

  applyEditorHeights(el, heights) {
    if (!heights) return;
    if (heights.minHeight) el.style.minHeight = heights.minHeight;
    if (heights.height) el.style.height = heights.height;
    if (heights.maxHeight) el.style.maxHeight = heights.maxHeight;
  }

  setupContainer() {
    initTextExpander(this.container.querySelector('text-expander'));
    this.container.addEventListener('ce-editor-content-changed', (e) => this.options?.onContentChanged?.(this, e));
  }

  setupTextarea() {
    this.textarea = this.container.querySelector('.markdown-text-editor');
    this.textarea._giteaMarkdownEditor = this;
    this.textarea.id = `_combo_markdown_editor_${String(elementIdCounter++)}`;
    this.textarea.addEventListener('input', (e) => this.options?.onContentChanged?.(this, e));
    this.applyEditorHeights(this.textarea, this.options.editorHeights);

    if (this.textarea.getAttribute('data-disable-autosize') !== 'true') {
      this.textareaAutosize = autosize(this.textarea, {viewportMarginBottom: 130});
    }

    this.textareaMarkdownToolbar = this.container.querySelector('markdown-toolbar');
    this.textareaMarkdownToolbar.setAttribute('for', this.textarea.id);
    for (const el of this.textareaMarkdownToolbar.querySelectorAll('.markdown-toolbar-button')) {
      // upstream bug: The role code is never executed in base MarkdownButtonElement https://github.com/github/markdown-toolbar-element/issues/70
      el.setAttribute('role', 'button');
      // the editor usually is in a form, so the buttons should have "type=button", avoiding conflicting with the form's submit.
      if (el.nodeName === 'BUTTON' && !el.getAttribute('type')) el.setAttribute('type', 'button');
    }

    this.textarea.addEventListener('keydown', (e) => {
      if (e.shiftKey) {
        e.target._shiftDown = true;
      }
    });
    this.textarea.addEventListener('keyup', (e) => {
      if (!e.shiftKey) {
        e.target._shiftDown = false;
      }
    });

    const monospaceButton = this.container.querySelector('.markdown-switch-monospace');
    const monospaceEnabled = localStorage?.getItem('markdown-editor-monospace') === 'true';
    const monospaceText = monospaceButton.getAttribute(monospaceEnabled ? 'data-disable-text' : 'data-enable-text');
    monospaceButton.setAttribute('data-tooltip-content', monospaceText);
    monospaceButton.setAttribute('aria-checked', String(monospaceEnabled));

    monospaceButton?.addEventListener('click', (e) => {
      e.preventDefault();
      const enabled = localStorage?.getItem('markdown-editor-monospace') !== 'true';
      localStorage.setItem('markdown-editor-monospace', String(enabled));
      this.textarea.classList.toggle('tw-font-mono', enabled);
      const text = monospaceButton.getAttribute(enabled ? 'data-disable-text' : 'data-enable-text');
      monospaceButton.setAttribute('data-tooltip-content', text);
      monospaceButton.setAttribute('aria-checked', String(enabled));
    });

    if (this.dropzone) {
      initTextareaPaste(this.textarea, this.dropzone);
    }
  }

  setupDropzone() {
    const dropzoneParentContainer = this.container.getAttribute('data-dropzone-parent-container');
    if (dropzoneParentContainer) {
      this.dropzone = this.container.closest(this.container.getAttribute('data-dropzone-parent-container'))?.querySelector('.dropzone');
    }
  }

  setupTab() {
    const $container = $(this.container);
    const tabs = $container[0].querySelectorAll('.tabular.menu > .item');

    // Fomantic Tab requires the "data-tab" to be globally unique.
    // So here it uses our defined "data-tab-for" and "data-tab-panel" to generate the "data-tab" attribute for Fomantic.
    const tabEditor = Array.from(tabs).find((tab) => tab.getAttribute('data-tab-for') === 'markdown-writer');
    const tabPreviewer = Array.from(tabs).find((tab) => tab.getAttribute('data-tab-for') === 'markdown-previewer');
    tabEditor.setAttribute('data-tab', `markdown-writer-${elementIdCounter}`);
    tabPreviewer.setAttribute('data-tab', `markdown-previewer-${elementIdCounter}`);
    const panelEditor = $container[0].querySelector('.ui.tab[data-tab-panel="markdown-writer"]');
    const panelPreviewer = $container[0].querySelector('.ui.tab[data-tab-panel="markdown-previewer"]');
    panelEditor.setAttribute('data-tab', `markdown-writer-${elementIdCounter}`);
    panelPreviewer.setAttribute('data-tab', `markdown-previewer-${elementIdCounter}`);
    elementIdCounter++;

    tabEditor.addEventListener('click', () => {
      requestAnimationFrame(() => {
        this.focus();
      });
    });

    $(tabs).tab();

    this.previewUrl = tabPreviewer.getAttribute('data-preview-url');
    this.previewContext = tabPreviewer.getAttribute('data-preview-context');
    this.previewMode = this.options.previewMode ?? 'comment';
    this.previewWiki = this.options.previewWiki ?? false;
    tabPreviewer.addEventListener('click', async () => {
      const formData = new FormData();
      formData.append('mode', this.previewMode);
      formData.append('context', this.previewContext);
      formData.append('text', this.value());
      formData.append('wiki', this.previewWiki);
      const response = await POST(this.previewUrl, {data: formData});
      const data = await response.text();
      renderPreviewPanelContent($(panelPreviewer), data);
    });
  }

  value(v = undefined) {
    if (v === undefined) {
      return this.textarea.value;
    }

    this.textarea.value = v;
    this.textareaAutosize?.resizeToFit();
  }

  focus() {
    this.textarea.focus();
  }

  moveCursorToEnd() {
    this.textarea.focus();
    this.textarea.setSelectionRange(this.textarea.value.length, this.textarea.value.length);
  }
}

export async function initMarkdownEditor(container, options = {}) {
  if (container instanceof $) {
    if (container.length !== 1) {
      throw new Error('initMarkdownEditor: container must be a single element');
    }
    container = container[0];
  }
  if (!container) {
    throw new Error('initMarkdownEditor: container is null');
  }
  const editor = new MarkdownEditor(container, options);
  await editor.init();
  return editor;
}

export function getMarkdownEditor(el) {
  if (el instanceof $) el = el[0];
  return el?._giteaMarkdownEditor;
}
