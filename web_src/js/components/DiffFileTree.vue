<template>
  <!-- eslint-disable -->
  <div
    v-show="visibilityInfo.isVisible"
    id="diff-file-tree"
    class="large screen only five wide column diff-detail-box sticky"
  >
    <div class="ui list">
      <DiffFileTreeItem :children="this.fileTree"></DiffFileTreeItem>
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
            isFile,
            level: index,
            file,
          };

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
        const fileBoxesDiv = document.querySelector('#diff-file-boxes');
        if (val.isVisible === true) {
          localStorage.setItem(LOCAL_STORAGE_KEY, 'true');
          fileBoxesDiv.classList.value =
            'sixteen wide mobile tablet eleven wide large screen column';
        } else {
          localStorage.setItem(LOCAL_STORAGE_KEY, 'false');
          fileBoxesDiv.classList.value = 'sixteen wide column';
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
    document
      .querySelector('.diff-show-file-tree-button')
      .addEventListener('click', (evt) => {
        evt.stopPropagation();
        this.visibilityInfo.isVisible = !this.visibilityInfo.isVisible;
      });
  },

  unmounted() {},

  methods: {},
};
</script>

<style scoped>
div.list {
  padding-top: 0;
}
</style>
