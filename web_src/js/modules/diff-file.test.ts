// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {diffTreeStoreSetViewed, reactiveDiffTreeStore} from './diff-file.ts';
import {describe, expect, it} from 'vitest';

describe('diff-file', () => {
  describe('reactiveDiffTreeStore', () => {
    it('should create store with provided data', () => {
      const store = reactiveDiffTreeStore({
        TreeRoot: {
          FullName: '',
          DisplayName: '',
          EntryMode: 'tree',
          IsViewed: false,
          NameHash: '',
          DiffStatus: '',
          FileIcon: '',
          Children: [],
        },
      }, '<svg>folder</svg>', '<svg>folder-open</svg>');

      expect(store.folderIcon).toBe('<svg>folder</svg>');
      expect(store.folderOpenIcon).toBe('<svg>folder-open</svg>');
      expect(store.fileTreeIsVisible).toBe(false);
      expect(store.selectedItem).toBe('');
    });

    it('should build fullNameMap correctly', () => {
      const store = reactiveDiffTreeStore({
        TreeRoot: {
          FullName: '',
          DisplayName: '',
          EntryMode: 'tree',
          IsViewed: false,
          NameHash: '',
          DiffStatus: '',
          FileIcon: '',
          Children: [
            {
              FullName: 'file1.txt',
              DisplayName: 'file1.txt',
              EntryMode: 'blob',
              IsViewed: false,
              NameHash: 'hash1',
              DiffStatus: 'added',
              FileIcon: '<svg></svg>',
              Children: null,
            },
            {
              FullName: 'src',
              DisplayName: 'src',
              EntryMode: 'tree',
              IsViewed: false,
              NameHash: 'hash2',
              DiffStatus: '',
              FileIcon: '<svg></svg>',
              Children: [
                {
                  FullName: 'src/file2.txt',
                  DisplayName: 'file2.txt',
                  EntryMode: 'blob',
                  IsViewed: false,
                  NameHash: 'hash3',
                  DiffStatus: 'modified',
                  FileIcon: '<svg></svg>',
                  Children: null,
                },
              ],
            },
          ],
        },
      }, '', '');

      expect(store.fullNameMap['file1.txt']).toBeDefined();
      expect(store.fullNameMap['src']).toBeDefined();
      expect(store.fullNameMap['src/file2.txt']).toBeDefined();
      expect(store.fullNameMap['nonexistent']).toBeUndefined();
    });

    it('should set ParentEntry for child entries', () => {
      const store = reactiveDiffTreeStore({
        TreeRoot: {
          FullName: '',
          DisplayName: '',
          EntryMode: 'tree',
          IsViewed: false,
          NameHash: '',
          DiffStatus: '',
          FileIcon: '',
          Children: [
            {
              FullName: 'src',
              DisplayName: 'src',
              EntryMode: 'tree',
              IsViewed: false,
              NameHash: 'hash1',
              DiffStatus: '',
              FileIcon: '<svg></svg>',
              Children: [
                {
                  FullName: 'src/file.txt',
                  DisplayName: 'file.txt',
                  EntryMode: 'blob',
                  IsViewed: false,
                  NameHash: 'hash2',
                  DiffStatus: 'added',
                  FileIcon: '<svg></svg>',
                  Children: null,
                },
              ],
            },
          ],
        },
      }, '', '');

      const srcEntry = store.fullNameMap['src'];
      const fileEntry = store.fullNameMap['src/file.txt'];

      expect(srcEntry.ParentEntry).toBe(store.diffFileTree.TreeRoot);
      expect(fileEntry.ParentEntry).toBe(srcEntry);
    });
  });

  describe('diffTreeStoreSetViewed', () => {
    it('should mark file as viewed', () => {
      const store = reactiveDiffTreeStore({
        TreeRoot: {
          FullName: '',
          DisplayName: '',
          EntryMode: 'tree',
          IsViewed: false,
          NameHash: '',
          DiffStatus: '',
          FileIcon: '',
          Children: [
            {
              FullName: 'test.txt',
              DisplayName: 'test.txt',
              EntryMode: 'blob',
              IsViewed: false,
              NameHash: 'hash1',
              DiffStatus: 'added',
              FileIcon: '',
              Children: null,
            },
          ],
        },
      }, '', '');

      diffTreeStoreSetViewed(store, 'test.txt', true);

      expect(store.fullNameMap['test.txt'].IsViewed).toBe(true);
    });

    it('should mark parent directory as viewed when all children are viewed', () => {
      const store = reactiveDiffTreeStore({
        TreeRoot: {
          FullName: '',
          DisplayName: '',
          EntryMode: 'tree',
          IsViewed: false,
          NameHash: '',
          DiffStatus: '',
          FileIcon: '',
          Children: [
            {
              FullName: 'dir1',
              DisplayName: 'dir1',
              EntryMode: 'tree',
              IsViewed: false,
              NameHash: 'hash1',
              DiffStatus: '',
              FileIcon: '',
              Children: [
                {
                  FullName: 'dir1/test.txt',
                  DisplayName: 'test.txt',
                  EntryMode: 'blob',
                  IsViewed: false,
                  NameHash: 'hash2',
                  DiffStatus: 'added',
                  FileIcon: '',
                  Children: null,
                },
              ],
            },
          ],
        },
      }, '', '');

      // Mark the only child as viewed
      diffTreeStoreSetViewed(store, 'dir1/test.txt', true);

      // Parent directory should also be marked as viewed
      expect(store.fullNameMap['dir1'].IsViewed).toBe(true);
    });

    it('should mark nested parent directories as viewed when all descendants are viewed', () => {
      const store = reactiveDiffTreeStore({
        TreeRoot: {
          FullName: '',
          DisplayName: '',
          EntryMode: 'tree',
          IsViewed: false,
          NameHash: '',
          DiffStatus: '',
          FileIcon: '',
          Children: [
            {
              FullName: 'dir1',
              DisplayName: 'dir1',
              EntryMode: 'tree',
              IsViewed: false,
              NameHash: 'hash1',
              DiffStatus: '',
              FileIcon: '',
              Children: [
                {
                  FullName: 'dir1/dir2',
                  DisplayName: 'dir2',
                  EntryMode: 'tree',
                  IsViewed: false,
                  NameHash: 'hash2',
                  DiffStatus: '',
                  FileIcon: '',
                  Children: [
                    {
                      FullName: 'dir1/dir2/file.txt',
                      DisplayName: 'file.txt',
                      EntryMode: 'blob',
                      IsViewed: false,
                      NameHash: 'hash3',
                      DiffStatus: 'added',
                      FileIcon: '',
                      Children: null,
                    },
                  ],
                },
              ],
            },
          ],
        },
      }, '', '');

      // Mark the deeply nested file as viewed
      diffTreeStoreSetViewed(store, 'dir1/dir2/file.txt', true);

      // All parent directories should be marked as viewed
      expect(store.fullNameMap['dir1/dir2/file.txt'].IsViewed).toBe(true);
      expect(store.fullNameMap['dir1/dir2'].IsViewed).toBe(true);
      expect(store.fullNameMap['dir1'].IsViewed).toBe(true);
    });

    it('should not mark parent as viewed when some children are not viewed', () => {
      const store = reactiveDiffTreeStore({
        TreeRoot: {
          FullName: '',
          DisplayName: '',
          EntryMode: 'tree',
          IsViewed: false,
          NameHash: '',
          DiffStatus: '',
          FileIcon: '',
          Children: [
            {
              FullName: 'dir1',
              DisplayName: 'dir1',
              EntryMode: 'tree',
              IsViewed: false,
              NameHash: 'hash1',
              DiffStatus: '',
              FileIcon: '',
              Children: [
                {
                  FullName: 'dir1/file1.txt',
                  DisplayName: 'file1.txt',
                  EntryMode: 'blob',
                  IsViewed: false,
                  NameHash: 'hash2',
                  DiffStatus: 'added',
                  FileIcon: '',
                  Children: null,
                },
                {
                  FullName: 'dir1/file2.txt',
                  DisplayName: 'file2.txt',
                  EntryMode: 'blob',
                  IsViewed: false,
                  NameHash: 'hash3',
                  DiffStatus: 'modified',
                  FileIcon: '',
                  Children: null,
                },
              ],
            },
          ],
        },
      }, '', '');

      // Mark only one child as viewed
      diffTreeStoreSetViewed(store, 'dir1/file1.txt', true);

      // Parent directory should NOT be marked as viewed
      expect(store.fullNameMap['dir1/file1.txt'].IsViewed).toBe(true);
      expect(store.fullNameMap['dir1/file2.txt'].IsViewed).toBe(false);
      expect(store.fullNameMap['dir1'].IsViewed).toBe(false);
    });

    it('should do nothing for non-existent entries', () => {
      const store = reactiveDiffTreeStore({
        TreeRoot: {
          FullName: '',
          DisplayName: '',
          EntryMode: 'tree',
          IsViewed: false,
          NameHash: '',
          DiffStatus: '',
          FileIcon: '',
          Children: [],
        },
      }, '', '');

      // Should not throw for non-existent entry
      expect(() => diffTreeStoreSetViewed(store, 'nonexistent.txt', true)).not.toThrow();
    });

    it('should mark file as not viewed when setting viewed to false', () => {
      const store = reactiveDiffTreeStore({
        TreeRoot: {
          FullName: '',
          DisplayName: '',
          EntryMode: 'tree',
          IsViewed: false,
          NameHash: '',
          DiffStatus: '',
          FileIcon: '',
          Children: [
            {
              FullName: 'test.txt',
              DisplayName: 'test.txt',
              EntryMode: 'blob',
              IsViewed: true,
              NameHash: 'hash1',
              DiffStatus: 'added',
              FileIcon: '',
              Children: null,
            },
          ],
        },
      }, '', '');

      // Initially viewed
      expect(store.fullNameMap['test.txt'].IsViewed).toBe(true);

      // Mark as not viewed
      diffTreeStoreSetViewed(store, 'test.txt', false);

      expect(store.fullNameMap['test.txt'].IsViewed).toBe(false);
    });
  });

  describe('Multiple files in same directory', () => {
    it('should only mark parent as viewed when ALL files are viewed', () => {
      const store = reactiveDiffTreeStore({
        TreeRoot: {
          FullName: '',
          DisplayName: '',
          EntryMode: 'tree',
          IsViewed: false,
          NameHash: '',
          DiffStatus: '',
          FileIcon: '',
          Children: [
            {
              FullName: 'dir1',
              DisplayName: 'dir1',
              EntryMode: 'tree',
              IsViewed: false,
              NameHash: 'hash1',
              DiffStatus: '',
              FileIcon: '',
              Children: [
                {
                  FullName: 'dir1/file1.txt',
                  DisplayName: 'file1.txt',
                  EntryMode: 'blob',
                  IsViewed: false,
                  NameHash: 'hash2',
                  DiffStatus: 'added',
                  FileIcon: '',
                  Children: null,
                },
                {
                  FullName: 'dir1/file2.txt',
                  DisplayName: 'file2.txt',
                  EntryMode: 'blob',
                  IsViewed: false,
                  NameHash: 'hash3',
                  DiffStatus: 'added',
                  FileIcon: '',
                  Children: null,
                },
                {
                  FullName: 'dir1/file3.txt',
                  DisplayName: 'file3.txt',
                  EntryMode: 'blob',
                  IsViewed: false,
                  NameHash: 'hash4',
                  DiffStatus: 'added',
                  FileIcon: '',
                  Children: null,
                },
              ],
            },
          ],
        },
      }, '', '');

      // View first two files
      diffTreeStoreSetViewed(store, 'dir1/file1.txt', true);
      diffTreeStoreSetViewed(store, 'dir1/file2.txt', true);

      // Parent should NOT be viewed yet
      expect(store.fullNameMap['dir1'].IsViewed).toBe(false);

      // View third file
      diffTreeStoreSetViewed(store, 'dir1/file3.txt', true);

      // Now parent should be viewed
      expect(store.fullNameMap['dir1'].IsViewed).toBe(true);
    });
  });
});