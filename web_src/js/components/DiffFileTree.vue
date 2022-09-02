<template>
  <div
    v-show="visibilityInfo.isVisible"
    id="diff-file-tree"
    class="mr-3 mt-3 diff-detail-box"
  >
    <div class="ui list">
      <DiffFileTreeItem v-for="item in fileTree" :key="item.name" :item="item" />
    </div>
    <div v-if="isIncomplete" id="diff-too-many-files-stats" class="pt-2">
      <span>{{ tooManyFilesMessage }}</span>
    </div>
  </div>
</template>

<script>
import DiffFileTreeItem from './DiffFileTreeItem.vue';

const {pageData} = window.config;
const LOCAL_STORAGE_KEY = 'diff_file_tree_visible';

export default {
  name: 'PullRequestFileTree',
  components: {DiffFileTreeItem},

  data: () => ({
    files: pageData.pullRequestFileTree.files,
    isIncomplete: pageData.pullRequestFileTree.isIncomplete,
    visibilityInfo: pageData.pullRequestFileTree.visibilityInfo,
    tooManyFilesMessage: pageData.pullRequestFileTree.tooManyFilesMessage
  }),

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
    },
  },

  watch: {
    visibilityInfo: {
      handler(val) {
        if (val.isVisible === true) {
          localStorage.setItem(LOCAL_STORAGE_KEY, 'true');
        } else {
          localStorage.setItem(LOCAL_STORAGE_KEY, 'false');
        }
      },
      deep: true,
    },
  },

  created() {
    const savedVisibility = localStorage.getItem(LOCAL_STORAGE_KEY);
    if (savedVisibility === 'true') {
      this.visibilityInfo.isVisible = true;
    }
  },

  mounted() {
    // ensure correct buttons when we are mounted to the dom
    this.toggleExpandCollapseButtons(this.visibilityInfo.isVisible);

    // Add our eventlisteners to the show / hide buttons
    document
      .querySelector('.diff-show-file-tree-button')
      .addEventListener('click', (evt) => {
        evt.stopPropagation();
        this.visibilityInfo.isVisible = !this.visibilityInfo.isVisible;
        this.toggleExpandCollapseButtons(this.visibilityInfo.isVisible);
      });
    document
      .querySelector('.diff-hide-file-tree-button')
      .addEventListener('click', (evt) => {
        evt.stopPropagation();
        this.visibilityInfo.isVisible = !this.visibilityInfo.isVisible;
        this.toggleExpandCollapseButtons(this.visibilityInfo.isVisible);
      });
  },

  methods: {
    toggleExpandCollapseButtons(isCurrentlyVisible) {
      document.querySelector('.diff-show-file-tree-button').style.display = isCurrentlyVisible ? 'none' : 'block';
      document.querySelector('.diff-hide-file-tree-button').style.display = isCurrentlyVisible ? 'block' : 'none';
    }
  },
};
</script>

<style scoped>
div.list {
  padding-top: 0;
}
</style>
