<template>
  <div
    v-if="fileTreeIsVisible"
    class="gt-mr-3 gt-mt-3 diff-detail-box"
  >
    <!-- only render the tree if we're visible. in many cases this is something that doesn't change very often -->
    <div class="ui list">
      <DiffFileTreeItem v-for="item in fileTree" :key="item.name" :item="item"/>
    </div>
    <div v-if="isIncomplete" id="diff-too-many-files-stats" class="gt-pt-2">
      <span class="gt-mr-2">{{ tooManyFilesMessage }}</span><a :class="['ui', 'basic', 'tiny', 'button', isLoadingNewData === true ? 'disabled' : '']" id="diff-show-more-files-stats" @click.stop="loadMoreData">{{ showMoreMessage }}</a>
    </div>
  </div>
</template>

<script>
import DiffFileTreeItem from './DiffFileTreeItem.vue';
import {doLoadMoreFiles} from '../features/repo-diff.js';
import {toggleElem} from '../utils/dom.js';
import {DiffTreeStore} from '../modules/stores.js';
import {setFileFolding} from '../features/file-fold.js';

const {pageData} = window.config;
const LOCAL_STORAGE_KEY = 'diff_file_tree_visible';

export default {
  components: {DiffFileTreeItem},
  data: () => {
    const fileTreeIsVisible = localStorage.getItem(LOCAL_STORAGE_KEY) === 'true';
    pageData.diffFileInfo.fileTreeIsVisible = fileTreeIsVisible;
    return {
      ...pageData.diffFileInfo,
      store: DiffTreeStore,
    };
  },
  computed: {
    fileTree() {
      const result = [];
      for (const file of this.files) {
        // Split file into directories
        const splits = file.Name.split('/');
        let index = 0;
        let parent = null;
        let isFile = false;
        for (const split of splits) {
          index += 1;
          // reached the end
          if (index === splits.length) {
            isFile = true;
          }
          let newParent = {
            name: split,
            children: [],
            isFile
          };

          if (isFile === true) {
            newParent.file = file;
          }

          if (parent) {
            // check if the folder already exists
            const existingFolder = parent.children.find(
              (x) => x.name === split
            );
            if (existingFolder) {
              newParent = existingFolder;
            } else {
              parent.children.push(newParent);
            }
          } else {
            const existingFolder = result.find((x) => x.name === split);
            if (existingFolder) {
              newParent = existingFolder;
            } else {
              result.push(newParent);
            }
          }
          parent = newParent;
        }
      }
      const mergeChildIfOnlyOneDir = (entries) => {
        for (const entry of entries) {
          if (entry.children) {
            mergeChildIfOnlyOneDir(entry.children);
          }
          if (entry.children.length === 1 && entry.children[0].isFile === false) {
            // Merge it to the parent
            entry.name = `${entry.name}/${entry.children[0].name}`;
            entry.children = entry.children[0].children;
          }
        }
      };
      // Merge folders with just a folder as children in order to
      // reduce the depth of our tree.
      mergeChildIfOnlyOneDir(result);
      return result;
    }
  },
  mounted() {
    // replace the pageData.diffFileInfo.files with our watched data so we get updates
    pageData.diffFileInfo.files = this.files;

    document.querySelector('.diff-toggle-file-tree-button').addEventListener('click', this.toggleVisibility);

    this.hashChangeListener = () => {
      this.store.selectedItem = window.location.hash;
      this.expandSelectedFile();
    };
    this.hashChangeListener();
    window.addEventListener('hashchange', this.hashChangeListener);
  },
  unmounted() {
    document.querySelector('.diff-toggle-file-tree-button').removeEventListener('click', this.toggleVisibility);
    window.removeEventListener('hashchange', this.hashChangeListener);
  },
  methods: {
    expandSelectedFile() {
      // expand file if the selected file is folded
      if (this.store.selectedItem) {
        const box = document.querySelector(this.store.selectedItem);
        const folded = box?.getAttribute('data-folded') === 'true';
        if (folded) setFileFolding(box, box.querySelector('.fold-file'), false);
      }
    },
    toggleVisibility() {
      this.updateVisibility(!this.fileTreeIsVisible);
    },
    updateVisibility(visible) {
      this.fileTreeIsVisible = visible;
      localStorage.setItem(LOCAL_STORAGE_KEY, this.fileTreeIsVisible);
      this.updateState(this.fileTreeIsVisible);
    },
    updateState(visible) {
      const btn = document.querySelector('.diff-toggle-file-tree-button');
      const [toShow, toHide] = btn.querySelectorAll('.icon');
      const tree = document.getElementById('diff-file-tree');
      const newTooltip = btn.getAttribute(visible ? 'data-hide-text' : 'data-show-text');
      btn.setAttribute('data-tooltip-content', newTooltip);
      toggleElem(tree, visible);
      toggleElem(toShow, !visible);
      toggleElem(toHide, visible);
    },
    loadMoreData() {
      this.isLoadingNewData = true;
      doLoadMoreFiles(this.link, this.diffEnd, () => {
        this.isLoadingNewData = false;
        const {pageData} = window.config;
        this.diffEnd = pageData.diffFileInfo.diffEnd;
      });
    },
  },
};
</script>
