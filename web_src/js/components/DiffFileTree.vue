<template>
  <div
    v-if="fileTreeIsVisible"
    class="mr-3 mt-3 diff-detail-box"
  >
    <!-- only render the tree if we're visible. in many cases this is something that doesn't change very often -->
    <div class="ui list">
      <DiffFileTreeItem v-for="item in fileTree" :key="item.name" :item="item" />
    </div>
    <div v-if="isIncomplete" id="diff-too-many-files-stats" class="pt-2">
      <span>{{ tooManyFilesMessage }}</span><a :class="['ui', 'basic', 'tiny', 'button', isLoadingNewData === true ? 'disabled' : '']" id="diff-show-more-files-stats" @click.stop="loadMoreData">{{ showMoreMessage }}</a>
    </div>
  </div>
</template>

<script>
import DiffFileTreeItem from './DiffFileTreeItem.vue';
import {doLoadMoreFiles} from '../features/repo-diff.js';

const {pageData} = window.config;
const LOCAL_STORAGE_KEY = 'diff_file_tree_visible';

export default {
  name: 'DiffFileTree',
  components: {DiffFileTreeItem},

  data: () => {
    const fileTreeIsVisible = localStorage.getItem(LOCAL_STORAGE_KEY) === 'true';
    pageData.diffFileInfo.fileTreeIsVisible = fileTreeIsVisible;
    return pageData.diffFileInfo;
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
    // ensure correct buttons when we are mounted to the dom
    this.adjustToggleButton(this.fileTreeIsVisible);
    document.querySelector('.diff-toggle-file-tree-button').addEventListener('click', this.toggleVisibility);
  },
  unmounted() {
    document.querySelector('.diff-toggle-file-tree-button').removeEventListener('click', this.toggleVisibility);
  },
  methods: {
    toggleVisibility() {
      this.updateVisibility(!this.fileTreeIsVisible);
    },
    updateVisibility(visible) {
      this.fileTreeIsVisible = visible;
      localStorage.setItem(LOCAL_STORAGE_KEY, this.fileTreeIsVisible);
      this.adjustToggleButton(this.fileTreeIsVisible);
    },
    adjustToggleButton(visible) {
      const [toShow, toHide] = document.querySelectorAll('.diff-toggle-file-tree-button .icon');
      toShow.classList.toggle('hide', visible);  // hide the toShow icon if the tree is visible
      toHide.classList.toggle('hide', !visible); // similarly

      const diffTree = document.getElementById('diff-file-tree');
      diffTree.classList.toggle('hide', !visible);
    },
    loadMoreData() {
      this.isLoadingNewData = true;
      doLoadMoreFiles(this.link, this.diffEnd, () => {
        this.isLoadingNewData = false;
      });
    }
  },
};
</script>
