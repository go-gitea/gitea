<template>
  <div
    v-show="fileTreeIsVisible"
    id="diff-file-tree"
    class="mr-3 mt-3 diff-detail-box"
  >
    <!-- only render the tree if we're visible. in many cases this is something that doesn't change very often -->
    <div class="ui list" v-if="fileTreeIsVisible">
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
      return result;
    }
  },

  mounted() {
    // ensure correct buttons when we are mounted to the dom
    this.adjustShowHideButtons(this.fileTreeIsVisible);

    // Add our eventlisteners to the show / hide buttons
    document
      .querySelector('.diff-show-file-tree-button')
      .addEventListener('click', (evt) => {
        evt.stopPropagation();
        this.toggleVisibility(this.fileTreeIsVisible);
      });
    document
      .querySelector('.diff-hide-file-tree-button')
      .addEventListener('click', (evt) => {
        evt.stopPropagation();
        this.toggleVisibility(this.fileTreeIsVisible);
      });
  },

  methods: {
    toggleVisibility(isCurrentlyVisible) {
      this.fileTreeIsVisible = !isCurrentlyVisible; // toggle the visibility
      localStorage.setItem(LOCAL_STORAGE_KEY, this.fileTreeIsVisible);
      this.adjustShowHideButtons(this.fileTreeIsVisible);
    },
    adjustShowHideButtons(isCurrentlyVisible) {
      document.querySelector('.diff-show-file-tree-button').style.display = isCurrentlyVisible ? 'none' : 'block';
      document.querySelector('.diff-hide-file-tree-button').style.display = isCurrentlyVisible ? 'block' : 'none';
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

<style scoped>
div.list {
  padding-top: 0;
}
</style>
