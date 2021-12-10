import attachTribute from '../tribute.js';

/**
 * create an EasyMDE editor for comment
 * @param textarea jQuery or HTMLElement
 * @returns {null|EasyMDE}
 */
export function createCommentEasyMDE(textarea) {
  if (textarea instanceof jQuery) {
    textarea = textarea[0];
  }
  if (!textarea) {
    return null;
  }

  const easyMDE = new window.EasyMDE({
    autoDownloadFontAwesome: false,
    element: textarea,
    forceSync: true,
    renderingConfig: {
      singleLineBreaks: false
    },
    indentWithTabs: false,
    tabSize: 4,
    spellChecker: false,
    toolbar: ['bold', 'italic', 'strikethrough', '|',
      'heading-1', 'heading-2', 'heading-3', 'heading-bigger', 'heading-smaller', '|',
      'code', 'quote', '|', {
        name: 'checkbox-empty',
        action(e) {
          const cm = e.codemirror;
          cm.replaceSelection(`\n- [ ] ${cm.getSelection()}`);
          cm.focus();
        },
        className: 'fa fa-square-o',
        title: 'Add Checkbox (empty)',
      },
      {
        name: 'checkbox-checked',
        action(e) {
          const cm = e.codemirror;
          cm.replaceSelection(`\n- [x] ${cm.getSelection()}`);
          cm.focus();
        },
        className: 'fa fa-check-square-o',
        title: 'Add Checkbox (checked)',
      }, '|',
      'unordered-list', 'ordered-list', '|',
      'link', 'image', 'table', 'horizontal-rule', '|',
      'clean-block', '|',
      {
        name: 'revert-to-textarea',
        action(e) {
          e.toTextArea();
        },
        className: 'fa fa-file',
        title: 'Revert to simple textarea',
      },
    ]
  });
  const inputField = easyMDE.codemirror.getInputField();
  inputField.classList.add('js-quick-submit');
  easyMDE.codemirror.setOption('extraKeys', {
    Enter: () => {
      const tributeContainer = document.querySelector('.tribute-container');
      if (!tributeContainer || tributeContainer.style.display === 'none') {
        return CodeMirror.Pass;
      }
    },
    Backspace: (cm) => {
      if (cm.getInputField().trigger) {
        cm.getInputField().trigger('input');
      }
      cm.execCommand('delCharBefore');
    }
  });
  attachTribute(inputField, {mentions: true, emoji: true});

  // TODO: that's the only way we can do now to attach the EasyMDE object to a HTMLElement
  inputField._data_easyMDE = easyMDE;
  textarea._data_easyMDE = easyMDE;
  return easyMDE;
}

/**
 * get the attached EasyMDE editor created by createCommentEasyMDE
 * @param el jQuery or HTMLElement
 * @returns {null|EasyMDE}
 */
export function getAttachedEasyMDE(el) {
  if (el instanceof jQuery) {
    el = el[0];
  }
  if (!el) {
    return null;
  }
  return el._data_easyMDE;
}
