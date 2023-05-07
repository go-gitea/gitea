import {svg} from '../../svg.js';

export function easyMDEToolbarActions(EasyMDE, editor) {
  const actions = {
    '|': '|',
    'heading-1': {
      action: EasyMDE.toggleHeading1,
      icon: svg('octicon-heading'),
      title: 'Heading 1',
    },
    'heading-2': {
      action: EasyMDE.toggleHeading2,
      icon: svg('octicon-heading'),
      title: 'Heading 2',
    },
    'heading-3': {
      action: EasyMDE.toggleHeading3,
      icon: svg('octicon-heading'),
      title: 'Heading 3',
    },
    'heading-smaller': {
      action: EasyMDE.toggleHeadingSmaller,
      icon: svg('octicon-heading'),
      title: 'Decrease Heading',
    },
    'heading-bigger': {
      action: EasyMDE.toggleHeadingBigger,
      icon: svg('octicon-heading'),
      title: 'Increase Heading',
    },
    'bold': {
      action: EasyMDE.toggleBold,
      icon: svg('octicon-bold'),
      title: 'Bold',
    },
    'italic': {
      action: EasyMDE.toggleItalic,
      icon: svg('octicon-italic'),
      title: 'Italic',
    },
    'strikethrough': {
      action: EasyMDE.toggleStrikethrough,
      icon: svg('octicon-strikethrough'),
      title: 'Strikethrough',
    },
    'quote': {
      action: EasyMDE.toggleBlockquote,
      icon: svg('octicon-quote'),
      title: 'Quote',
    },
    'code': {
      action: EasyMDE.toggleCodeBlock,
      icon: svg('octicon-code'),
      title: 'Code',
    },
    'link': {
      action: EasyMDE.drawLink,
      icon: svg('octicon-link'),
      title: 'Link',
    },
    'unordered-list': {
      action: EasyMDE.toggleUnorderedList,
      icon: svg('octicon-list-unordered'),
      title: 'Unordered List',
    },
    'ordered-list': {
      action: EasyMDE.toggleOrderedList,
      icon: svg('octicon-list-ordered'),
      title: 'Ordered List',
    },
    'image': {
      action: EasyMDE.drawImage,
      icon: svg('octicon-image'),
      title: 'Image',
    },
    'table': {
      action: EasyMDE.drawTable,
      icon: svg('octicon-table'),
      title: 'Table',
    },
    'horizontal-rule': {
      action: EasyMDE.drawHorizontalRule,
      icon: svg('octicon-horizontal-rule'),
      title: 'Horizontal Rule',
    },
    'preview': {
      action: EasyMDE.togglePreview,
      icon: svg('octicon-eye'),
      title: 'Preview',
    },
    'fullscreen': {
      action: EasyMDE.toggleFullScreen,
      icon: svg('octicon-screen-full'),
      title: 'Fullscreen',
    },
    'side-by-side': {
      action: EasyMDE.toggleSideBySide,
      icon: svg('octicon-columns'),
      title: 'Side by Side',
    },

    // gitea's custom actions
    'gitea-checkbox-empty': {
      action(e) {
        const cm = e.codemirror;
        cm.replaceSelection(`\n- [ ] ${cm.getSelection()}`);
        cm.focus();
      },
      icon: svg('gitea-empty-checkbox'),
      title: 'Add Checkbox (empty)',
    },
    'gitea-checkbox-checked': {
      action(e) {
        const cm = e.codemirror;
        cm.replaceSelection(`\n- [x] ${cm.getSelection()}`);
        cm.focus();
      },
      icon: svg('octicon-checkbox'),
      title: 'Add Checkbox (checked)',
    },
    'gitea-switch-to-textarea': {
      action: () => {
        editor.userPreferredEditor = 'textarea';
        editor.switchToTextarea();
      },
      icon: svg('octicon-arrow-switch'),
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
      icon: svg('octicon-chevron-right'),
      title: 'Add Inline Code',
    }
  };

  for (const [key, value] of Object.entries(actions)) {
    if (typeof value !== 'string') {
      value.name = key;
    }
  }

  return actions;
}
