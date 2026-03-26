// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {describe, expect, it, vi, beforeEach} from 'vitest';
import {createViewFileTreeStore} from './ViewFileTreeStore.ts';

// Mock the GET function
vi.mock('../modules/fetch.ts', () => ({
  GET: vi.fn(),
}));

// Mock dom utils
vi.mock('../utils/dom.ts', () => ({
  createElementFromHTML: vi.fn(() => document.createElement('div')),
}));

// Mock html utils
vi.mock('../utils/html.ts', () => ({
  html: (strings: TemplateStringsArray) => strings.join(''),
}));

describe('ViewFileTreeStore', () => {
  const mockProps = {
    repoLink: '/owner/repo',
    treePath: '',
    currentRefNameSubURL: 'branch/main',
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Store creation', () => {
    it('should create a store with initial state', () => {
      const store = createViewFileTreeStore(mockProps);

      expect(store.rootFiles).toEqual([]);
      expect(store.selectedItem).toBe(mockProps.treePath);
    });

    it('should have all required methods', () => {
      const store = createViewFileTreeStore(mockProps);

      expect(typeof store.loadChildren).toBe('function');
      expect(typeof store.loadViewContent).toBe('function');
      expect(typeof store.navigateTreeView).toBe('function');
      expect(typeof store.buildTreePathWebUrl).toBe('function');
    });
  });

  describe('buildTreePathWebUrl', () => {
    it('should build correct URL for a file path', () => {
      const store = createViewFileTreeStore(mockProps);

      const url = store.buildTreePathWebUrl('src/components/Test.ts');

      expect(url).toBe('/owner/repo/src/branch/main/src/components/Test.ts');
    });

    it('should handle empty path', () => {
      const store = createViewFileTreeStore(mockProps);

      const url = store.buildTreePathWebUrl('');

      expect(url).toBe('/owner/repo/src/branch/main/');
    });

    it('should handle special characters in path', () => {
      const store = createViewFileTreeStore(mockProps);

      const url = store.buildTreePathWebUrl('file with spaces.txt');

      // Path segments should be URL encoded
      expect(url).toContain('file%20with%20spaces.txt');
    });
  });

  describe('selectedItem', () => {
    it('should start with treePath as selectedItem', () => {
      const propsWithTreePath = {
        ...mockProps,
        treePath: 'src/components',
      };

      const store = createViewFileTreeStore(propsWithTreePath);

      expect(store.selectedItem).toBe('src/components');
    });

    it('should update selectedItem on navigateTreeView', async () => {
      // Mock window.origin
      const originalOrigin = window.origin;
      Object.defineProperty(window, 'origin', {
        value: 'http://localhost:3000',
        writable: true,
      });

      // Mock document.querySelector for repo-view-content
      const mockElement = document.createElement('div');
      mockElement.className = 'repo-view-content';
      const mockDataElement = document.createElement('div');
      mockDataElement.className = 'repo-view-content-data';
      mockDataElement.setAttribute('data-document-title', 'Test');
      mockDataElement.setAttribute('data-document-title-common', 'Gitea');
      mockElement.appendChild(mockDataElement);
      document.body.appendChild(mockElement);

      const {GET} = await import('../modules/fetch.ts');
      const mockResponse = {
        text: vi.fn().mockResolvedValue('<div class="repo-view-content-data" data-document-title="Test" data-document-title-common="Gitea"></div>'),
      };
      vi.mocked(GET).mockResolvedValue(mockResponse as any);

      const store = createViewFileTreeStore(mockProps);

      // Mock history.pushState
      const pushStateSpy = vi.spyOn(window.history, 'pushState').mockImplementation(() => {});

      await store.navigateTreeView('src/test.ts');

      expect(store.selectedItem).toBe('src/test.ts');

      pushStateSpy.mockRestore();
      Object.defineProperty(window, 'origin', {
        value: originalOrigin,
        writable: true,
      });

      // Cleanup
      document.body.removeChild(mockElement);
    });
  });

  describe('loadChildren', () => {
    it('should return null when no ref name is provided', async () => {
      const propsWithoutRef = {
        ...mockProps,
        currentRefNameSubURL: '',
      };

      const store = createViewFileTreeStore(propsWithoutRef);

      const result = await store.loadChildren('');

      expect(result).toBeNull();
    });

    it('should fetch and return file tree nodes', async () => {
      const {GET} = await import('../modules/fetch.ts');
      const mockResponse = {
        json: vi.fn().mockResolvedValue({
          fileTreeNodes: [
            {
              entryName: 'src',
              entryMode: 'tree',
              entryIcon: '<svg></svg>',
              fullPath: 'src',
            },
          ],
        }),
      };
      vi.mocked(GET).mockResolvedValue(mockResponse as any);

      const store = createViewFileTreeStore(mockProps);

      const result = await store.loadChildren('src');

      expect(result).toEqual([
        {
          entryName: 'src',
          entryMode: 'tree',
          entryIcon: '<svg></svg>',
          fullPath: 'src',
        },
      ]);
    });
  });

  describe('Reactivity', () => {
    it('should be reactive', () => {
      const store = createViewFileTreeStore(mockProps);

      // Initially empty
      expect(store.rootFiles).toEqual([]);

      // Update rootFiles
      store.rootFiles = [
        {
          entryName: 'README.md',
          entryMode: 'blob',
          entryIcon: '<svg></svg>',
          fullPath: 'README.md',
        },
      ];

      expect(store.rootFiles).toHaveLength(1);
      expect(store.rootFiles[0].entryName).toBe('README.md');
    });
  });
});