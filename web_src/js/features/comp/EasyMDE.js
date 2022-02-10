import $ from 'jquery';
import attachTribute from '../tribute.js';

const {appSubUrl} = window.config;

function loadScript(url) {
  return new Promise((resolve, reject) => {
    const script = document.createElement('script');
    script.async = true;
    script.addEventListener('load', () => {
      resolve();
    });
    script.addEventListener('error', (e) => {
      reject(e.error);
    });
    script.src = url;
    document.body.appendChild(script);
  });
}

/**
 * @returns {EasyMDE}
 */
export async function importEasyMDE() {
  // for CodeMirror: the plugins should be loaded dynamically
  // https://github.com/codemirror/CodeMirror/issues/5484
  // https://github.com/codemirror/CodeMirror/issues/4838

  // EasyMDE's CSS should be loaded via webpack config, otherwise our own styles can not overwrite the default styles.
  const [{default: EasyMDE}, {default: CodeMirror}] = await Promise.all([
    import(/* webpackChunkName: "easymde" */'easymde'),
    import(/* webpackChunkName: "codemirror" */'codemirror'),
  ]);

  // CodeMirror plugins must be loaded by a "Plain browser env"
  window.CodeMirror = CodeMirror;
  await Promise.all([
    loadScript(`${appSubUrl}/assets/vendor/plugins/codemirror/addon/mode/loadmode.js`),
    loadScript(`${appSubUrl}/assets/vendor/plugins/codemirror/mode/meta.js`),
  ]);

  // the loadmode.js/meta.js would set the modeURL/modeInfo properties, so we check it to make sure our loading works
  if (!CodeMirror.modeURL || !CodeMirror.modeInfo) {
    throw new Error('failed to load plugins for CodeMirror');
  }

  CodeMirror.modeURL = `${appSubUrl}/assets/vendor/plugins/codemirror/mode/%N/%N.js`;
  return EasyMDE;
}

/**
 * create an EasyMDE editor for comment
 * @param textarea jQuery or HTMLElement
 * @param easyMDEOptions the options for EasyMDE
 * @returns {null|EasyMDE}
 */
export async function createCommentEasyMDE(textarea, easyMDEOptions = {}) {
  if (textarea instanceof $) {
    textarea = textarea[0];
  }
  if (!textarea) {
    return null;
  }

  const EasyMDE = await importEasyMDE();

  const easyMDE = new EasyMDE({
    autoDownloadFontAwesome: false,
    element: textarea,
    forceSync: true,
    renderingConfig: {
      singleLineBreaks: false,
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
    ], ...easyMDEOptions});
  const inputField = easyMDE.codemirror.getInputField();
  inputField.classList.add('js-quick-submit');
  easyMDE.codemirror.setOption('extraKeys', {
    Enter: () => {
      const tributeContainer = document.querySelector('.tribute-container');
      if (!tributeContainer || tributeContainer.style.display === 'none') {
        return window.CodeMirror.Pass;
      }
    },
    Backspace: (cm) => {
      if (cm.getInputField().trigger) {
        cm.getInputField().trigger('input');
      }
      cm.execCommand('delCharBefore');
    },
  });
  attachTribute(inputField, {mentions: true, emoji: true});
  attachEasyMDEToElements(easyMDE);
  return easyMDE;
}

/**
 * attach the EasyMDE object to its input elements (InputField, TextArea)
 * @param {EasyMDE} easyMDE
 */
export function attachEasyMDEToElements(easyMDE) {
  // TODO: that's the only way we can do now to attach the EasyMDE object to a HTMLElement

  // InputField is used by CodeMirror to accept user input
  const inputField = easyMDE.codemirror.getInputField();
  inputField._data_easyMDE = easyMDE;

  // TextArea is the real textarea element in the form
  const textArea = easyMDE.codemirror.getTextArea();
  textArea._data_easyMDE = easyMDE;
}


/**
 * get the attached EasyMDE editor created by createCommentEasyMDE
 * @param el jQuery or HTMLElement
 * @returns {null|EasyMDE}
 */
export function getAttachedEasyMDE(el) {
  if (el instanceof $) {
    el = el[0];
  }
  if (!el) {
    return null;
  }
  return el._data_easyMDE;
}

/**
 * validate if the given EasyMDE textarea is is non-empty.
 * @param {jQuery} $textarea
 * @returns {boolean} returns true if validation succeeded.
 */
export function validateTextareaNonEmpty($textarea) {
  const $mdeInputField = $(getAttachedEasyMDE($textarea).codemirror.getInputField());
  // The original edit area HTML element is hidden and replaced by the
  // SimpleMDE/EasyMDE editor, breaking HTML5 input validation if the text area is empty.
  // This is a workaround for this upstream bug.
  // See https://github.com/sparksuite/simplemde-markdown-editor/issues/324
  if (!$textarea.val()) {
    $mdeInputField.prop('required', true);
    const $form = $textarea.parents('form');
    if (!$form.length) {
      // this should never happen. we put a alert here in case the textarea would be forgotten to be put in a form
      alert('Require non-empty content');
    } else {
      $form[0].reportValidity();
    }
    return false;
  }
  $mdeInputField.prop('required', false);
  return true;
}
