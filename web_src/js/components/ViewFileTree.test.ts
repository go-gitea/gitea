// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {mount} from '@vue/test-utils';
import {describe, expect, it, vi, beforeEach} from 'vitest';
import ViewFileTree from './ViewFileTree.vue';

// Mock the ViewFileTreeStore
vi.mock('./ViewFileTreeStore.ts', () => ({
  createViewFileTreeStore: vi.fn(),
}));

describe('ViewFileTree', () => {
  const createMockStore = (rootFiles: any[] = []) => ({
    rootFiles,
    selectedItem: '',
    loadChildren: vi.fn().mockResolvedValue([]),
    loadViewContent: vi.fn(),
    navigateTreeView: vi.fn(),
    buildTreePathWebUrl: vi.fn((path: string) => `/owner/repo/src/branch/main/${path}`),
  });

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render tree with correct ARIA attributes', async () => {
    const mockStore = createMockStore();
    const {createViewFileTreeStore} = await import('./ViewFileTreeStore.ts');
    vi.mocked(createViewFileTreeStore).mockReturnValue(mockStore as any);

    const wrapper = mount(ViewFileTree, {
      props: {
        repoLink: '/owner/repo',
        treePath: '',
        currentRefNameSubURL: 'branch/main',
      },
      global: {
        stubs: {
          ViewFileTreeItem: true,
        },
      },
    });

    // Check tree container has role="tree"
    const treeContainer = wrapper.find('.view-file-tree-items');
    expect(treeContainer.exists()).toBe(true);
    expect(treeContainer.attributes('role')).toBe('tree');

    // Check aria-label is present
    expect(treeContainer.attributes('aria-label')).toBe('File tree');
  });

  it('should render file items as children', async () => {
    const rootFiles = [
      {
        entryName: 'src',
        entryMode: 'tree',
        entryIcon: '<svg></svg>',
        entryIconOpen: '<svg></svg>',
        fullPath: 'src',
        children: [],
      },
      {
        entryName: 'README.md',
        entryMode: 'blob',
        entryIcon: '<svg></svg>',
        fullPath: 'README.md',
      },
    ];

    const mockStore = createMockStore(rootFiles);
    const {createViewFileTreeStore} = await import('./ViewFileTreeStore.ts');
    vi.mocked(createViewFileTreeStore).mockReturnValue(mockStore as any);

    const wrapper = mount(ViewFileTree, {
      props: {
        repoLink: '/owner/repo',
        treePath: '',
        currentRefNameSubURL: 'branch/main',
      },
      global: {
        stubs: {
          ViewFileTreeItem: {
            template: '<div class="tree-item-stub"></div>',
            props: ['item', 'store'],
          },
        },
      },
    });

    // Should render two tree items
    const treeItems = wrapper.findAllComponents({name: 'ViewFileTreeItem'});
    expect(treeItems.length).toBe(2);
  });
});