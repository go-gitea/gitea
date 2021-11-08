import attachTribute from '../tribute.js';

export function createCommentSimpleMDE($editArea) {
  if ($editArea.length === 0) {
    return null;
  }

  const simplemde = new SimpleMDE({
    autoDownloadFontAwesome: false,
    element: $editArea[0],
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
  $(simplemde.codemirror.getInputField()).addClass('js-quick-submit');
  simplemde.codemirror.setOption('extraKeys', {
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
  attachTribute(simplemde.codemirror.getInputField(), {mentions: true, emoji: true});
  $editArea.data('simplemde', simplemde);
  $(simplemde.codemirror.getInputField()).data('simplemde', simplemde);
  return simplemde;
}
