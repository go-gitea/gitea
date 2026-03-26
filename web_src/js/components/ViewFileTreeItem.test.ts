// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {mount} from '@vue/test-utils';
import {describe, expect, it, vi} from 'vitest';
import ViewFileTreeItem from './ViewFileTreeItem.vue';
import type {FileTreeItem} from './ViewFileTreeStore.ts';

// Mock SvgIcon component
vi.mock('../svg.ts', () => ({
  SvgIcon: {
    name: 'SvgIcon',
    template: '<span class="svg-icon-mock"></span>',
    props: ['name', 'size', 'class'],
  },
}));

// Mock dom utils
vi.mock('../utils/dom.ts', () => ({
  isPlainClick: vi.fn(() => true),
}));

// Mock are-you-sure
vi.mock('../vendor/jquery.are-you-sure.ts', () => ({
  shouldTriggerAreYouSure: vi.fn(() => false),
}));

const createMockStore = () => ({
  rootFiles: [],
  selectedItem: '',
  loadChildren: vi.fn().mockResolvedValue([]),
  loadViewContent: vi.fn(),
  navigateTreeView: vi.fn(),
  buildTreePathWebUrl: vi.fn((path: string) => `/owner/repo/src/branch/main/${path}`),
});

const createDirectoryItem = (): FileTreeItem => ({
  entryName: 'src',
  entryMode: 'tree',
  entryIcon: '<svg></svg>',
  entryIconOpen: '<svg></svg>',
  fullPath: 'src',
  children: [],
});

const createFileItem = (): FileTreeItem => ({
  entryName: 'README.md',
  entryMode: 'blob',
  entryIcon: '<svg></svg>',
  fullPath: 'README.md',
});

const createSubmoduleItem = (): FileTreeItem => ({
  entryName: 'vendor',
  entryMode: 'commit',
  entryIcon: '<svg></svg>',
  fullPath: 'vendor',
  submoduleUrl: 'https://github.com/example/repo',
});

const createSymlinkItem = (): FileTreeItem => ({
  entryName: 'link',
  entryMode: 'symlink',
  entryIcon: '<svg></svg>',
  fullPath: 'link',
});

describe('ViewFileTreeItem', () => {
  describe('ARIA attributes', () => {
    it('should have role="treeitem" on all items', () => {
      const mockStore = createMockStore();

      // Test directory item
      const dirWrapper = mount(ViewFileTreeItem, {
        props: {
          item: createDirectoryItem(),
          store: mockStore,
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const dirElement = dirWrapper.find('.tree-item');
      expect(dirElement.attributes('role')).toBe('treeitem');

      // Test file item
      const fileWrapper = mount(ViewFileTreeItem, {
        props: {
          item: createFileItem(),
          store: mockStore,
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const fileElement = fileWrapper.find('.tree-item');
      expect(fileElement.attributes('role')).toBe('treeitem');
    });

    it('should have aria-expanded on directory items', () => {
      const mockStore = createMockStore();

      const wrapper = mount(ViewFileTreeItem, {
        props: {
          item: createDirectoryItem(),
          store: mockStore,
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const treeItem = wrapper.find('.tree-item');
      expect(treeItem.attributes('aria-expanded')).toBeDefined();
    });

    it('should not have aria-expanded on file items', () => {
      const mockStore = createMockStore();

      const wrapper = mount(ViewFileTreeItem, {
        props: {
          item: createFileItem(),
          store: mockStore,
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const treeItem = wrapper.find('.tree-item');
      expect(treeItem.attributes('aria-expanded')).toBeUndefined();
    });

    it('should have aria-selected="true" when item is selected', () => {
      const mockStore = createMockStore();
      mockStore.selectedItem = 'README.md';

      const wrapper = mount(ViewFileTreeItem, {
        props: {
          item: createFileItem(),
          store: mockStore,
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const treeItem = wrapper.find('.tree-item');
      expect(treeItem.attributes('aria-selected')).toBe('true');
    });

    it('should have aria-selected="false" when item is not selected', () => {
      const mockStore = createMockStore();
      mockStore.selectedItem = 'other.txt';

      const wrapper = mount(ViewFileTreeItem, {
        props: {
          item: createFileItem(),
          store: mockStore,
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const treeItem = wrapper.find('.tree-item');
      expect(treeItem.attributes('aria-selected')).toBe('false');
    });

    it('should have role="group" on sub-items container', async () => {
      const mockStore = createMockStore();
      mockStore.loadChildren.mockResolvedValue([
        {
          entryName: 'file.txt',
          entryMode: 'blob',
          entryIcon: '<svg></svg>',
          fullPath: 'src/file.txt',
        },
      ]);

      const item = createDirectoryItem();
      item.children = [
        {
          entryName: 'file.txt',
          entryMode: 'blob',
          entryIcon: '<svg></svg>',
          fullPath: 'src/file.txt',
        },
      ];

      const wrapper = mount(ViewFileTreeItem, {
        props: {
          item,
          store: mockStore,
        },
        global: {
          stubs: {
            SvgIcon: true,
            ViewFileTreeItem: true,
          },
        },
      });

      // Check sub-items has role="group"
      const subItems = wrapper.find('.sub-items');
      expect(subItems.exists()).toBe(true);
      expect(subItems.attributes('role')).toBe('group');
    });
  });

  describe('CSS classes for item types', () => {
    it('should have "type-directory" class for directories', () => {
      const mockStore = createMockStore();

      const wrapper = mount(ViewFileTreeItem, {
        props: {
          item: createDirectoryItem(),
          store: mockStore,
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const treeItem = wrapper.find('.tree-item');
      expect(treeItem.classes()).toContain('type-directory');
    });

    it('should have "type-file" class for blob files', () => {
      const mockStore = createMockStore();

      const wrapper = mount(ViewFileTreeItem, {
        props: {
          item: createFileItem(),
          store: mockStore,
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const treeItem = wrapper.find('.tree-item');
      expect(treeItem.classes()).toContain('type-file');
    });

    it('should have "type-file" class for exec files', () => {
      const mockStore = createMockStore();

      const execItem: FileTreeItem = {
        entryName: 'script.sh',
        entryMode: 'exec',
        entryIcon: '<svg></svg>',
        fullPath: 'script.sh',
      };

      const wrapper = mount(ViewFileTreeItem, {
        props: {
          item: execItem,
          store: mockStore,
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const treeItem = wrapper.find('.tree-item');
      expect(treeItem.classes()).toContain('type-file');
    });

    it('should have "type-submodule" class for submodules', () => {
      const mockStore = createMockStore();

      const wrapper = mount(ViewFileTreeItem, {
        props: {
          item: createSubmoduleItem(),
          store: mockStore,
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const treeItem = wrapper.find('.tree-item');
      expect(treeItem.classes()).toContain('type-submodule');
    });

    it('should have "type-symlink" class for symlinks', () => {
      const mockStore = createMockStore();

      const wrapper = mount(ViewFileTreeItem, {
        props: {
          item: createSymlinkItem(),
          store: mockStore,
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const treeItem = wrapper.find('.tree-item');
      expect(treeItem.classes()).toContain('type-symlink');
    });
  });

  describe('Selection state', () => {
    it('should have "selected" class when item is selected', () => {
      const mockStore = createMockStore();
      mockStore.selectedItem = 'README.md';

      const wrapper = mount(ViewFileTreeItem, {
        props: {
          item: createFileItem(),
          store: mockStore,
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const treeItem = wrapper.find('.tree-item');
      expect(treeItem.classes()).toContain('selected');
    });

    it('should not have "selected" class when item is not selected', () => {
      const mockStore = createMockStore();
      mockStore.selectedItem = 'other.txt';

      const wrapper = mount(ViewFileTreeItem, {
        props: {
          item: createFileItem(),
          store: mockStore,
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const treeItem = wrapper.find('.tree-item');
      expect(treeItem.classes()).not.toContain('selected');
    });
  });
});