import {removeAttachmentLinksFromMarkdown} from './EditorUpload.ts';

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
