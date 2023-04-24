import '@github/markdown-toolbar-element';
import '@github/text-expander-element';
import $ from 'jquery';
import {attachTribute} from '../tribute.js';
import {hideElem, showElem, autosize} from '../../utils/dom.js';
import {initEasyMDEImagePaste, initTextareaImagePaste} from './ImagePaste.js';
import {handleGlobalEnterQuickSubmit} from './QuickSubmit.js';
import {emojiString} from '../emoji.js';
import {renderPreviewPanelContent} from '../repo-editor.js';
import {matchEmoji, matchMention} from '../../utils/match.js';

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
    this.prepareEasyMDEToolbarActions();
    this.setupTab();
    this.setupDropzone();
    this.setupTextarea();
    this.setupExpander();

    if (this.userPreferredEditor === 'easymde') {
      await this.switchToEasyMDE();
    }
  }

  applyEditorHeights(el, heights) {
    if (!heights) return;
    if (heights.minHeight) el.style.minHeight = heights.minHeight;
    if (heights.height) el.style.height = heights.height;
    if (heights.maxHeight) el.style.maxHeight = heights.maxHeight;
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

  setupExpander() {
    const expander = this.container.querySelector('text-expander');
    expander?.addEventListener('text-expander-change', ({detail: {key, provide, text}}) => {
      if (key === ':') {
        const matches = matchEmoji(text);
        if (!matches.length) return provide({matched: false});

        const ul = document.createElement('ul');
        ul.classList.add('suggestions');
        for (const name of matches) {
          const emoji = emojiString(name);
          const li = document.createElement('li');
          li.setAttribute('role', 'option');
          li.setAttribute('data-value', emoji);
          li.textContent = `${emoji} ${name}`;
          ul.append(li);
        }

        provide({matched: true, fragment: ul});
      } else if (key === '@') {
        const matches = matchMention(text);
        if (!matches.length) return provide({matched: false});

        const ul = document.createElement('ul');
        ul.classList.add('suggestions');
        for (const {value, name, fullname, avatar} of matches) {
          const li = document.createElement('li');
          li.setAttribute('role', 'option');
          li.setAttribute('data-value', `${key}${value}`);

          const img = document.createElement('img');
          img.src = avatar;
          li.append(img);

          const nameSpan = document.createElement('span');
          nameSpan.textContent = name;
          li.append(nameSpan);

          if (fullname && fullname.toLowerCase() !== name) {
            const fullnameSpan = document.createElement('span');
            fullnameSpan.classList.add('fullname');
            fullnameSpan.textContent = fullname;
            li.append(fullnameSpan);
          }

          ul.append(li);
        }

        provide({matched: true, fragment: ul});
      }
    });
    expander?.addEventListener('text-expander-value', ({detail}) => {
      if (detail?.item) {
        detail.value = detail.item.getAttribute('data-value');
      }
    });
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
        action: () => {
          this.userPreferredEditor = 'textarea';
          this.switchToTextarea();
        },
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

  switchToTextarea() {
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
