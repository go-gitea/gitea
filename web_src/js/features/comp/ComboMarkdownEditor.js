import '@github/markdown-toolbar-element';
import '@github/text-expander-element';
import $ from 'jquery';
import {attachTribute} from '../tribute.js';
import {hideElem, showElem, autosize} from '../../utils/dom.js';
import {initEasyMDEImagePaste, initTextareaImagePaste} from './ImagePaste.js';
import {handleGlobalEnterQuickSubmit} from './QuickSubmit.js';
import {renderPreviewPanelContent} from '../repo-editor.js';
import {easyMDEToolbarActions} from './EasyMDEToolbarActions.js';
import {initTextExpander} from './TextExpander.js';
import {showErrorToast} from '../../modules/toast.js';

let elementIdCounter = 0;

/**
 * validate if the given textarea is non-empty.
 * @param {jQuery} $textarea
 * @returns {boolean} returns true if validation succeeded.
 */
export function validateTextareaNonEmpty($textarea) {
  // When using EasyMDE, the original edit area HTML element is hidden, breaking HTML5 input validation.
  // The workaround (https://github.com/sparksuite/simplemde-markdown-editor/issues/324) doesn't work with contenteditable, so we just show an alert.
  if (!$textarea.val()) {
    if ($textarea.is(':visible')) {
      $textarea.prop('required', true);
      const $form = $textarea.parents('form');
      $form[0]?.reportValidity();
    } else {
      // The alert won't hurt users too much, because we are dropping the EasyMDE and the check only occurs in a few places.
      showErrorToast('Require non-empty content');
    }
    return false;
  }
  return true;
}

class ComboMarkdownEditor {
  constructor(container, options = {}) {
    container._giteaComboMarkdownEditor = this;
    this.options = options;
    this.container = container;
  }

  async init() {
    this.prepareEasyMDEToolbarActions();
    this.setupContainer();
    this.setupTab();
    this.setupDropzone();
    this.setupTextarea();

    await this.switchToUserPreference();
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
    this.textarea._giteaComboMarkdownEditor = this;
    this.textarea.id = `_combo_markdown_editor_${String(elementIdCounter++)}`;
    this.textarea.addEventListener('input', (e) => this.options?.onContentChanged?.(this, e));
    this.applyEditorHeights(this.textarea, this.options.editorHeights);
    this.textareaAutosize = autosize(this.textarea, {viewportMarginBottom: 130});

    this.textareaMarkdownToolbar = this.container.querySelector('markdown-toolbar');
    this.textareaMarkdownToolbar.setAttribute('for', this.textarea.id);
    for (const el of this.textareaMarkdownToolbar.querySelectorAll('.markdown-toolbar-button')) {
      // upstream bug: The role code is never executed in base MarkdownButtonElement https://github.com/github/markdown-toolbar-element/issues/70
      el.setAttribute('role', 'button');
    }

    const monospaceButton = this.container.querySelector('.markdown-switch-monospace');
    const monospaceEnabled = localStorage?.getItem('markdown-editor-monospace') === 'true';
    const monospaceText = monospaceButton.getAttribute(monospaceEnabled ? 'data-disable-text' : 'data-enable-text');
    monospaceButton.setAttribute('data-tooltip-content', monospaceText);
    monospaceButton.setAttribute('aria-checked', String(monospaceEnabled));

    monospaceButton?.addEventListener('click', (e) => {
      e.preventDefault();
      const enabled = localStorage?.getItem('markdown-editor-monospace') !== 'true';
      localStorage.setItem('markdown-editor-monospace', String(enabled));
      this.textarea.classList.toggle('gt-mono', enabled);
      const text = monospaceButton.getAttribute(enabled ? 'data-disable-text' : 'data-enable-text');
      monospaceButton.setAttribute('data-tooltip-content', text);
      monospaceButton.setAttribute('aria-checked', String(enabled));
    });

    const easymdeButton = this.container.querySelector('.markdown-switch-easymde');
    easymdeButton?.addEventListener('click', async (e) => {
      e.preventDefault();
      this.userPreferredEditor = 'easymde';
      await this.switchToEasyMDE();
    });

    if (this.dropzone) {
      initTextareaImagePaste(this.textarea, this.dropzone);
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
    const $tabMenu = $container.find('.tabular.menu');
    const $tabs = $tabMenu.find('> .item');

    // Fomantic Tab requires the "data-tab" to be globally unique.
    // So here it uses our defined "data-tab-for" and "data-tab-panel" to generate the "data-tab" attribute for Fomantic.
    const $tabEditor = $tabs.filter(`.item[data-tab-for="markdown-writer"]`);
    const $tabPreviewer = $tabs.filter(`.item[data-tab-for="markdown-previewer"]`);
    $tabEditor.attr('data-tab', `markdown-writer-${elementIdCounter}`);
    $tabPreviewer.attr('data-tab', `markdown-previewer-${elementIdCounter}`);
    const $panelEditor = $container.find('.ui.tab[data-tab-panel="markdown-writer"]');
    const $panelPreviewer = $container.find('.ui.tab[data-tab-panel="markdown-previewer"]');
    $panelEditor.attr('data-tab', `markdown-writer-${elementIdCounter}`);
    $panelPreviewer.attr('data-tab', `markdown-previewer-${elementIdCounter}`);
    elementIdCounter++;

    $tabs.tab();

    this.previewUrl = $tabPreviewer.attr('data-preview-url');
    this.previewContext = $tabPreviewer.attr('data-preview-context');
    this.previewMode = this.options.previewMode ?? 'comment';
    this.previewWiki = this.options.previewWiki ?? false;
    $tabPreviewer.on('click', () => {
      $.post(this.previewUrl, {
        _csrf: window.config.csrfToken,
        mode: this.previewMode,
        context: this.previewContext,
        text: this.value(),
        wiki: this.previewWiki,
      }, (data) => {
        renderPreviewPanelContent($panelPreviewer, data);
      });
    });
  }

  prepareEasyMDEToolbarActions() {
    this.easyMDEToolbarDefault = [
      'bold', 'italic', 'strikethrough', '|', 'heading-1', 'heading-2', 'heading-3',
      'heading-bigger', 'heading-smaller', '|', 'code', 'quote', '|', 'gitea-checkbox-empty',
      'gitea-checkbox-checked', '|', 'unordered-list', 'ordered-list', '|', 'link', 'image',
      'table', 'horizontal-rule', '|', 'gitea-switch-to-textarea',
    ];
  }

  parseEasyMDEToolbar(EasyMDE, actions) {
    this.easyMDEToolbarActions = this.easyMDEToolbarActions || easyMDEToolbarActions(EasyMDE, this);
    const processed = [];
    for (const action of actions) {
      const actionButton = this.easyMDEToolbarActions[action];
      if (!actionButton) throw new Error(`Unknown EasyMDE toolbar action ${action}`);
      processed.push(actionButton);
    }
    return processed;
  }

  async switchToUserPreference() {
    if (this.userPreferredEditor === 'easymde') {
      await this.switchToEasyMDE();
    } else {
      this.switchToTextarea();
    }
  }

  switchToTextarea() {
    if (!this.easyMDE) return;
    showElem(this.textareaMarkdownToolbar);
    if (this.easyMDE) {
      this.easyMDE.toTextArea();
      this.easyMDE = null;
    }
  }

  async switchToEasyMDE() {
    if (this.easyMDE) return;
    // EasyMDE's CSS should be loaded via webpack config, otherwise our own styles can not overwrite the default styles.
    const {default: EasyMDE} = await import(/* webpackChunkName: "easymde" */'easymde');
    const easyMDEOpt = {
      autoDownloadFontAwesome: false,
      element: this.textarea,
      forceSync: true,
      renderingConfig: {singleLineBreaks: false},
      indentWithTabs: false,
      tabSize: 4,
      spellChecker: false,
      inputStyle: 'contenteditable', // nativeSpellcheck requires contenteditable
      nativeSpellcheck: true,
      ...this.options.easyMDEOptions,
    };
    easyMDEOpt.toolbar = this.parseEasyMDEToolbar(EasyMDE, easyMDEOpt.toolbar ?? this.easyMDEToolbarDefault);

    this.easyMDE = new EasyMDE(easyMDEOpt);
    this.easyMDE.codemirror.on('change', (...args) => {this.options?.onContentChanged?.(this, ...args)});
    this.easyMDE.codemirror.setOption('extraKeys', {
      'Cmd-Enter': (cm) => handleGlobalEnterQuickSubmit(cm.getTextArea()),
      'Ctrl-Enter': (cm) => handleGlobalEnterQuickSubmit(cm.getTextArea()),
      Enter: (cm) => {
        const tributeContainer = document.querySelector('.tribute-container');
        if (!tributeContainer || tributeContainer.style.display === 'none') {
          cm.execCommand('newlineAndIndent');
        }
      },
      Up: (cm) => {
        const tributeContainer = document.querySelector('.tribute-container');
        if (!tributeContainer || tributeContainer.style.display === 'none') {
          return cm.execCommand('goLineUp');
        }
      },
      Down: (cm) => {
        const tributeContainer = document.querySelector('.tribute-container');
        if (!tributeContainer || tributeContainer.style.display === 'none') {
          return cm.execCommand('goLineDown');
        }
      },
    });
    this.applyEditorHeights(this.container.querySelector('.CodeMirror-scroll'), this.options.editorHeights);
    await attachTribute(this.easyMDE.codemirror.getInputField(), {mentions: true, emoji: true});
    initEasyMDEImagePaste(this.easyMDE, this.dropzone);
    hideElem(this.textareaMarkdownToolbar);
  }

  value(v = undefined) {
    if (v === undefined) {
      if (this.easyMDE) {
        return this.easyMDE.value();
      }
      return this.textarea.value;
    }

    if (this.easyMDE) {
      this.easyMDE.value(v);
    } else {
      this.textarea.value = v;
    }
    this.textareaAutosize.resizeToFit();
  }

  focus() {
    if (this.easyMDE) {
      this.easyMDE.codemirror.focus();
    } else {
      this.textarea.focus();
    }
  }

  moveCursorToEnd() {
    this.textarea.focus();
    this.textarea.setSelectionRange(this.textarea.value.length, this.textarea.value.length);
    if (this.easyMDE) {
      this.easyMDE.codemirror.focus();
      this.easyMDE.codemirror.setCursor(this.easyMDE.codemirror.lineCount(), 0);
    }
  }

  get userPreferredEditor() {
    return window.localStorage.getItem(`markdown-editor-${this.options.useScene ?? 'default'}`);
  }
  set userPreferredEditor(s) {
    window.localStorage.setItem(`markdown-editor-${this.options.useScene ?? 'default'}`, s);
  }
}

export function getComboMarkdownEditor(el) {
  if (el instanceof $) el = el[0];
  return el?._giteaComboMarkdownEditor;
}

export async function initComboMarkdownEditor(container, options = {}) {
  if (container instanceof $) {
    if (container.length !== 1) {
      throw new Error('initComboMarkdownEditor: container must be a single element');
    }
    container = container[0];
  }
  if (!container) {
    throw new Error('initComboMarkdownEditor: container is null');
  }
  const editor = new ComboMarkdownEditor(container, options);
  await editor.init();
  return editor;
}
