import '@github/markdown-toolbar-element';
import {attachTribute} from '../tribute.js';
import {hideElem, showElem} from '../../utils/dom.js';
import {initEasyMDEImagePaste, initTextareaImagePaste} from './ImagePaste.js';
import $ from 'jquery';
import {initMarkupContent} from '../../markup/content.js';
import {handleGlobalEnterQuickSubmit} from './QuickSubmit.js';
import {attachRefIssueContextPopup} from '../contextpopup.js';

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
      alert('Require non-empty content');
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
    this.textarea = this.container.querySelector('.markdown-text-editor');
    this.textarea._giteaComboMarkdownEditor = this;
    this.textarea.id = `_combo_markdown_editor_${String(elementIdCounter)}`;
    this.textarea.addEventListener('input', (e) => {this.options?.onContentChanged?.(this, e)});
    this.textareaMarkdownToolbar = this.container.querySelector('markdown-toolbar');
    this.textareaMarkdownToolbar.setAttribute('for', this.textarea.id);

    elementIdCounter++;

    this.switchToEasyMDEButton = this.container.querySelector('.markdown-switch-easymde');
    this.switchToEasyMDEButton?.addEventListener('click', async (e) => {
      e.preventDefault();
      await this.switchToEasyMDE();
    });

    await attachTribute(this.textarea, {mentions: true, emoji: true});

    const dropzoneParentContainer = this.container.getAttribute('data-dropzone-parent-container');
    if (dropzoneParentContainer) {
      this.dropzone = this.container.closest(this.container.getAttribute('data-dropzone-parent-container'))?.querySelector('.dropzone');
      initTextareaImagePaste(this.textarea, this.dropzone);
    }

    this.setupTab();
    this.prepareEasyMDEToolbarActions();
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
        $panelPreviewer.html(data);
        initMarkupContent();

        const refIssues = $panelPreviewer.find('p .ref-issue');
        attachRefIssueContextPopup(refIssues);
      });
    });
  }

  prepareEasyMDEToolbarActions() {
    this.easyMDEToolbarDefault = [
      'bold', 'italic', 'strikethrough', '|', 'heading-1', 'heading-2', 'heading-3', 'heading-bigger', 'heading-smaller', '|',
      'code', 'quote', '|', 'gitea-checkbox-empty', 'gitea-checkbox-checked', '|',
      'unordered-list', 'ordered-list', '|', 'link', 'image', 'table', 'horizontal-rule', '|', 'clean-block', '|',
      'gitea-switch-to-textarea',
    ];

    this.easyMDEToolbarActions = {
      'gitea-checkbox-empty': {
        action(e) {
          const cm = e.codemirror;
          cm.replaceSelection(`\n- [ ] ${cm.getSelection()}`);
          cm.focus();
        },
        className: 'fa fa-square-o',
        title: 'Add Checkbox (empty)',
      },
      'gitea-checkbox-checked': {
        action(e) {
          const cm = e.codemirror;
          cm.replaceSelection(`\n- [x] ${cm.getSelection()}`);
          cm.focus();
        },
        className: 'fa fa-check-square-o',
        title: 'Add Checkbox (checked)',
      },
      'gitea-switch-to-textarea': {
        action: this.switchToTextarea.bind(this),
        className: 'fa fa-file',
        title: 'Revert to simple textarea',
      },
      'gitea-code-inline': {
        action(e) {
          const cm = e.codemirror;
          const selection = cm.getSelection();
          cm.replaceSelection(`\`${selection}\``);
          if (!selection) {
            const cursorPos = cm.getCursor();
            cm.setCursor(cursorPos.line, cursorPos.ch - 1);
          }
          cm.focus();
        },
        className: 'fa fa-angle-right',
        title: 'Add Inline Code',
      }
    };
  }

  parseEasyMDEToolbar(actions) {
    const processed = [];
    for (const action of actions) {
      if (action.startsWith('gitea-')) {
        const giteaAction = this.easyMDEToolbarActions[action];
        if (!giteaAction) throw new Error(`Unknown EasyMDE toolbar action ${action}`);
        processed.push(giteaAction);
      } else {
        processed.push(action);
      }
    }
    return processed;
  }

  async switchToTextarea() {
    showElem(this.textareaMarkdownToolbar);
    if (this.easyMDE) {
      this.easyMDE.toTextArea();
      this.easyMDE = null;
    }
  }

  async switchToEasyMDE() {
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
    easyMDEOpt.toolbar = this.parseEasyMDEToolbar(easyMDEOpt.toolbar ?? this.easyMDEToolbarDefault);

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
