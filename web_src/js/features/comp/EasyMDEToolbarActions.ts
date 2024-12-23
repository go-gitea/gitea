import {svg} from '../../svg.ts';
import type EasyMDE from 'easymde';
import type {ComboMarkdownEditor} from './ComboMarkdownEditor.ts';

export function easyMDEToolbarActions(easyMde: typeof EasyMDE, editor: ComboMarkdownEditor): Record<string, Partial<EasyMDE.ToolbarIcon | string>> {
  const actions: Record<string, Partial<EasyMDE.ToolbarIcon> | string> = {
    '|': '|',
    'heading-1': {
      action: easyMde.toggleHeading1,
      icon: svg('octicon-heading'),
      title: 'Heading 1',
    },
    'heading-2': {
      action: easyMde.toggleHeading2,
      icon: svg('octicon-heading'),
      title: 'Heading 2',
    },
    'heading-3': {
      action: easyMde.toggleHeading3,
      icon: svg('octicon-heading'),
      title: 'Heading 3',
    },
    'heading-smaller': {
      action: easyMde.toggleHeadingSmaller,
      icon: svg('octicon-heading'),
      title: 'Decrease Heading',
    },
    'heading-bigger': {
      action: easyMde.toggleHeadingBigger,
      icon: svg('octicon-heading'),
      title: 'Increase Heading',
    },
    'bold': {
      action: easyMde.toggleBold,
      icon: svg('octicon-bold'),
      title: 'Bold',
    },
    'italic': {
      action: easyMde.toggleItalic,
      icon: svg('octicon-italic'),
      title: 'Italic',
    },
    'strikethrough': {
      action: easyMde.toggleStrikethrough,
      icon: svg('octicon-strikethrough'),
      title: 'Strikethrough',
    },
    'quote': {
      action: easyMde.toggleBlockquote,
      icon: svg('octicon-quote'),
      title: 'Quote',
    },
    'code': {
      action: easyMde.toggleCodeBlock,
      icon: svg('octicon-code'),
      title: 'Code',
    },
    'link': {
      action: easyMde.drawLink,
      icon: svg('octicon-link'),
      title: 'Link',
    },
    'unordered-list': {
      action: easyMde.toggleUnorderedList,
      icon: svg('octicon-list-unordered'),
      title: 'Unordered List',
    },
    'ordered-list': {
      action: easyMde.toggleOrderedList,
      icon: svg('octicon-list-ordered'),
      title: 'Ordered List',
    },
    'image': {
      action: easyMde.drawImage,
      icon: svg('octicon-image'),
      title: 'Image',
    },
    'table': {
      action: easyMde.drawTable,
      icon: svg('octicon-table'),
      title: 'Table',
    },
    'horizontal-rule': {
      action: easyMde.drawHorizontalRule,
      icon: svg('octicon-horizontal-rule'),
      title: 'Horizontal Rule',
    },
    'preview': {
      action: easyMde.togglePreview,
      icon: svg('octicon-eye'),
      title: 'Preview',
    },
    'fullscreen': {
      action: easyMde.toggleFullScreen,
      icon: svg('octicon-screen-full'),
      title: 'Fullscreen',
    },
    'side-by-side': {
      action: easyMde.toggleSideBySide,
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
    },
  };

  for (const [key, value] of Object.entries(actions)) {
    if (typeof value !== 'string') {
      value.name = key;
    }
  }

  return actions;
}
