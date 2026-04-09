import {formatFileSize} from './size.ts';

test('formatFileSize', () => {
  expect(formatFileSize(0)).toBe('0 B');
  expect(formatFileSize(512)).toBe('512 B');
  expect(formatFileSize(1024)).toBe('1.0 KiB');
  expect(formatFileSize(1536)).toBe('1.5 KiB');
  expect(formatFileSize(10 * 1024)).toBe('10 KiB');
  expect(formatFileSize(1024 * 1024)).toBe('1.0 MiB');
  expect(formatFileSize(1024 * 1024 * 1024)).toBe('1.0 GiB');
});
