import {pasteAsMarkdownLink, removeAttachmentLinksFromMarkdown} from './EditorUpload.ts';

test('removeAttachmentLinksFromMarkdown', () => {
  expect(removeAttachmentLinksFromMarkdown('a foo b', 'foo')).toBe('a foo b');
  expect(removeAttachmentLinksFromMarkdown('a [x](attachments/foo) b', 'foo')).toBe('a  b');
  expect(removeAttachmentLinksFromMarkdown('a ![x](attachments/foo) b', 'foo')).toBe('a  b');
  expect(removeAttachmentLinksFromMarkdown('a [x](/attachments/foo) b', 'foo')).toBe('a  b');
  expect(removeAttachmentLinksFromMarkdown('a ![x](/attachments/foo) b', 'foo')).toBe('a  b');

  expect(removeAttachmentLinksFromMarkdown('a <img src="attachments/foo"> b', 'foo')).toBe('a  b');
  expect(removeAttachmentLinksFromMarkdown('a <img width="100" src="attachments/foo"> b', 'foo')).toBe('a  b');
  expect(removeAttachmentLinksFromMarkdown('a <img src="/attachments/foo"> b', 'foo')).toBe('a  b');
  expect(removeAttachmentLinksFromMarkdown('a <img src="/attachments/foo" width="100"/> b', 'foo')).toBe('a  b');
});

test('preparePasteAsMarkdownLink', () => {
  expect(pasteAsMarkdownLink({value: 'foo', selectionStart: 0, selectionEnd: 0}, 'bar')).toBeNull();
  expect(pasteAsMarkdownLink({value: 'foo', selectionStart: 0, selectionEnd: 0}, 'https://gitea.com')).toBeNull();
  expect(pasteAsMarkdownLink({value: 'foo', selectionStart: 0, selectionEnd: 3}, 'bar')).toBeNull();
  expect(pasteAsMarkdownLink({value: 'foo', selectionStart: 0, selectionEnd: 3}, 'https://gitea.com')).toBe('[foo](https://gitea.com)');
  expect(pasteAsMarkdownLink({value: '..(url)', selectionStart: 3, selectionEnd: 6}, 'https://gitea.com')).toBe('[url](https://gitea.com)');
  expect(pasteAsMarkdownLink({value: '[](url)', selectionStart: 3, selectionEnd: 6}, 'https://gitea.com')).toBeNull();
  expect(pasteAsMarkdownLink({value: 'https://example.com', selectionStart: 0, selectionEnd: 19}, 'https://gitea.com')).toBeNull();
});
