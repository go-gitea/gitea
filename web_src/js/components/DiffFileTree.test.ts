// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {mount} from '@vue/test-utils';
import {describe, expect, it, vi, beforeEach} from 'vitest';
import DiffFileTree from './DiffFileTree.vue';

// Mock the diffTreeStore to return a test store
vi.mock('../modules/diff-file.ts', () => ({
  diffTreeStore: vi.fn(),
  reactiveDiffTreeStore: vi.fn(),
}));

// Mock localUserSettings
vi.mock('../modules/user-settings.ts', () => ({
  localUserSettings: {
    getBoolean: vi.fn(() => true),
    setBoolean: vi.fn(),
  },
}));

// Mock dom utils
vi.mock('../utils/dom.ts', () => ({
  toggleElem: vi.fn(),
}));

// Mock file-fold
vi.mock('../features/file-fold.ts', () => ({
  setFileFolding: vi.fn(),
}));

describe('DiffFileTree', () => {
  beforeEach(() => {
    // Create the required DOM elements
    document.body.innerHTML = `
      <div class="diff-toggle-file-tree-button" data-show-text="Show" data-hide-text="Hide">
        <span class="icon"></span>
        <span class="icon"></span>
      </div>
      <div id="diff-file-tree"></div>
    `;
    vi.clearAllMocks();
  });

  const createMockStore = (fileTreeIsVisible = true, children: any[] = []) => ({
    fileTreeIsVisible,
    diffFileTree: {
      TreeRoot: {
        FullName: '',
        DisplayName: '',
        EntryMode: 'tree',
        IsViewed: false,
        NameHash: '',
        DiffStatus: '',
        FileIcon: '',
        Children: children,
      },
    },
    selectedItem: '',
    folderIcon: '<svg></svg>',
    folderOpenIcon: '<svg></svg>',
  });

  it('should render tree with correct ARIA attributes', async () => {
    const mockStore = createMockStore(true);
    const {diffTreeStore} = await import('../modules/diff-file.ts');
    vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

    const wrapper = mount(DiffFileTree, {
      global: {
        stubs: {
          DiffFileTreeItem: true,
        },
      },
    });

    // Check tree container has role="tree"
    const treeContainer = wrapper.find('.diff-file-tree-items');
    expect(treeContainer.exists()).toBe(true);
    expect(treeContainer.attributes('role')).toBe('tree');

    // Check aria-label is present
    expect(treeContainer.attributes('aria-label')).toBe('Diff file tree');
  });

  it('should not render tree when fileTreeIsVisible is false', async () => {
    const mockStore = createMockStore(false);
    const {diffTreeStore} = await import('../modules/diff-file.ts');
    vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

    const wrapper = mount(DiffFileTree, {
      global: {
        stubs: {
          DiffFileTreeItem: true,
        },
      },
    });

    // Tree should not be visible
    const treeContainer = wrapper.find('.diff-file-tree-items');
    expect(treeContainer.exists()).toBe(false);
  });

  it('should render children as tree items', async () => {
    const children = [
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
        FullName: 'file2.txt',
        DisplayName: 'file2.txt',
        EntryMode: 'blob',
        IsViewed: false,
        NameHash: 'hash2',
        DiffStatus: 'modified',
        FileIcon: '<svg></svg>',
        Children: null,
      },
    ];

    const mockStore = createMockStore(true, children);
    const {diffTreeStore} = await import('../modules/diff-file.ts');
    vi.mocked(diffTreeStore).mockReturnValue(mockStore as any);

    const wrapper = mount(DiffFileTree, {
      global: {
        stubs: {
          DiffFileTreeItem: {
            template: '<div class="tree-item-stub"></div>',
            props: ['item'],
          },
        },
      },
    });

    // Should render two tree items
    const treeItems = wrapper.findAllComponents({name: 'DiffFileTreeItem'});
    expect(treeItems.length).toBe(2);
  });
});