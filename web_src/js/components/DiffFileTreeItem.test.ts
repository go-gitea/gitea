// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {mount} from '@vue/test-utils';
import {describe, expect, it, vi} from 'vitest';
import DiffFileTreeItem from './DiffFileTreeItem.vue';
import {diffTreeStore, type DiffTreeEntry} from '../modules/diff-file.ts';

// Mock the diffTreeStore
vi.mock('../modules/diff-file.ts', () => ({
  diffTreeStore: vi.fn(),
}));

// Mock SvgIcon component
vi.mock('../svg.ts', () => ({
  SvgIcon: {
    name: 'SvgIcon',
    template: '<span class="svg-icon-mock"></span>',
    props: ['name', 'size', 'class'],
  },
}));

describe('DiffFileTreeItem', () => {
  const createMockStore = (selectedItem = '') => ({
    selectedItem,
    folderIcon: '<svg class="folder"></svg>',
    folderOpenIcon: '<svg class="folder-open"></svg>',
    fullNameMap: {},
  });

  const createDirectoryItem = (): DiffTreeEntry => ({
    FullName: 'src',
    DisplayName: 'src',
    NameHash: 'hash-src',
    DiffStatus: '',
    EntryMode: 'tree',
    IsViewed: false,
    Children: [
      {
        FullName: 'src/file.txt',
        DisplayName: 'file.txt',
        NameHash: 'hash-file',
        DiffStatus: 'added',
        EntryMode: 'blob',
        IsViewed: false,
        Children: null,
        FileIcon: '<svg></svg>',
      },
    ],
    FileIcon: '<svg></svg>',
  });

  const createFileItem = (): DiffTreeEntry => ({
    FullName: 'test.txt',
    DisplayName: 'test.txt',
    NameHash: 'hash-test',
    DiffStatus: 'modified',
    EntryMode: 'blob',
    IsViewed: false,
    Children: null,
    FileIcon: '<svg></svg>',
  });

  describe('ARIA attributes for directories', () => {
    it('should have role="treeitem" on directory items', () => {
      const mockStore = createMockStore();
      vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

      const wrapper = mount(DiffFileTreeItem, {
        props: {
          item: createDirectoryItem(),
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const directoryElement = wrapper.find('.item-directory');
      expect(directoryElement.exists()).toBe(true);
      expect(directoryElement.attributes('role')).toBe('treeitem');
    });

    it('should have aria-expanded attribute on directories', () => {
      const mockStore = createMockStore();
      vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

      const wrapper = mount(DiffFileTreeItem, {
        props: {
          item: createDirectoryItem(),
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const directoryElement = wrapper.find('.item-directory');
      // aria-expanded should be present
      expect(directoryElement.attributes('aria-expanded')).toBeDefined();
    });

    it('should toggle aria-expanded when directory is collapsed/expanded', async () => {
      const mockStore = createMockStore();
      vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

      const wrapper = mount(DiffFileTreeItem, {
        props: {
          item: createDirectoryItem(),
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const directoryElement = wrapper.find('.item-directory');

      // Initially collapsed (based on item.IsViewed, which is false)
      // aria-expanded should be the opposite of collapsed
      const initialExpanded = directoryElement.attributes('aria-expanded');

      // Click to toggle
      await directoryElement.trigger('click');

      // aria-expanded should change
      const afterClickExpanded = directoryElement.attributes('aria-expanded');
      expect(afterClickExpanded).not.toBe(initialExpanded);
    });

    it('should have tabindex="0" on directory items for keyboard navigation', () => {
      const mockStore = createMockStore();
      vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

      const wrapper = mount(DiffFileTreeItem, {
        props: {
          item: createDirectoryItem(),
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const directoryElement = wrapper.find('.item-directory');
      expect(directoryElement.attributes('tabindex')).toBe('0');
    });

    it('should have role="group" on sub-items container', () => {
      const mockStore = createMockStore();
      vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

      const wrapper = mount(DiffFileTreeItem, {
        props: {
          item: createDirectoryItem(),
        },
        global: {
          stubs: {
            SvgIcon: true,
            DiffFileTreeItem: true,
          },
        },
      });

      // Expand the directory
      wrapper.find('.item-directory').trigger('click');

      const subItems = wrapper.find('.sub-items');
      expect(subItems.exists()).toBe(true);
      expect(subItems.attributes('role')).toBe('group');
    });
  });

  describe('Keyboard navigation for directories', () => {
    it('should toggle expansion on Enter key', async () => {
      const mockStore = createMockStore();
      vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

      const wrapper = mount(DiffFileTreeItem, {
        props: {
          item: createDirectoryItem(),
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const directoryElement = wrapper.find('.item-directory');
      const initialExpanded = directoryElement.attributes('aria-expanded');

      // Press Enter key
      await directoryElement.trigger('keydown.enter');

      const afterKeydownExpanded = directoryElement.attributes('aria-expanded');
      expect(afterKeydownExpanded).not.toBe(initialExpanded);
    });

    it('should toggle expansion on Space key', async () => {
      const mockStore = createMockStore();
      vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

      const wrapper = mount(DiffFileTreeItem, {
        props: {
          item: createDirectoryItem(),
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const directoryElement = wrapper.find('.item-directory');
      const initialExpanded = directoryElement.attributes('aria-expanded');

      // Press Space key
      await directoryElement.trigger('keydown.space');

      const afterKeydownExpanded = directoryElement.attributes('aria-expanded');
      expect(afterKeydownExpanded).not.toBe(initialExpanded);
    });
  });

  describe('ARIA attributes for files', () => {
    it('should have role="treeitem" on file items', () => {
      const mockStore = createMockStore();
      vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

      const wrapper = mount(DiffFileTreeItem, {
        props: {
          item: createFileItem(),
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const fileElement = wrapper.find('.item-file');
      expect(fileElement.exists()).toBe(true);
      expect(fileElement.attributes('role')).toBe('treeitem');
    });

    it('should have aria-selected="true" when item is selected', () => {
      const mockStore = createMockStore('#diff-hash-test');
      vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

      const wrapper = mount(DiffFileTreeItem, {
        props: {
          item: createFileItem(),
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const fileElement = wrapper.find('.item-file');
      expect(fileElement.attributes('aria-selected')).toBe('true');
    });

    it('should have aria-selected="false" when item is not selected', () => {
      const mockStore = createMockStore('#diff-other');
      vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

      const wrapper = mount(DiffFileTreeItem, {
        props: {
          item: createFileItem(),
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const fileElement = wrapper.find('.item-file');
      expect(fileElement.attributes('aria-selected')).toBe('false');
    });

    it('should have correct href for file items', () => {
      const mockStore = createMockStore();
      vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

      const wrapper = mount(DiffFileTreeItem, {
        props: {
          item: createFileItem(),
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const fileElement = wrapper.find('.item-file');
      expect(fileElement.attributes('href')).toBe('#diff-hash-test');
    });
  });

  describe('Viewed state styling', () => {
    it('should have "viewed" class when item is viewed', () => {
      const mockStore = createMockStore();
      vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

      const viewedItem = createFileItem();
      viewedItem.IsViewed = true;

      const wrapper = mount(DiffFileTreeItem, {
        props: {
          item: viewedItem,
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const fileElement = wrapper.find('.item-file');
      expect(fileElement.classes()).toContain('viewed');
    });

    it('should have "selected" class when item is selected', () => {
      const mockStore = createMockStore('#diff-hash-test');
      vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

      const wrapper = mount(DiffFileTreeItem, {
        props: {
          item: createFileItem(),
        },
        global: {
          stubs: {
            SvgIcon: true,
          },
        },
      });

      const fileElement = wrapper.find('.item-file');
      expect(fileElement.classes()).toContain('selected');
    });
  });
});