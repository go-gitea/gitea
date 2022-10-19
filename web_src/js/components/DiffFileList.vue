<template>
  <ol class="diff-detail-box diff-stats m-0" ref="root" v-if="fileListIsVisible">
    <li v-for="file in files" :key="file.NameHash">
      <div class="bold df ac pull-right">
        <span v-if="file.IsBin" class="ml-1 mr-3">{{ binaryFileMessage }}</span>
        {{ file.IsBin ? '' : file.Addition + file.Deletion }}
        <span v-if="!file.IsBin" class="diff-stats-bar tooltip mx-3" :data-content="statisticsMessage.replace('%d', (file.Addition + file.Deletion)).replace('%d', file.Addition).replace('%d', file.Deletion)">
          <div class="diff-stats-add-bar" :style="{ 'width': diffStatsWidth(file.Addition, file.Deletion) }" />
        </span>
      </div>
      <!-- todo finish all file status, now modify, add, delete and rename -->
      <span :class="['status', diffTypeToString(file.Type), 'tooltip']" :data-content="diffTypeToString(file.Type)" data-position="right center">&nbsp;</span>
      <a class="file mono" :href="'#diff-' + file.NameHash">{{ file.Name }}</a>
    </li>
    <li v-if="isIncomplete" id="diff-too-many-files-stats" class="pt-2">
      <span class="file df ac sb">{{ tooManyFilesMessage }}
        <a :class="['ui', 'basic', 'tiny', 'button', isLoadingNewData === true ? 'disabled' : '']" id="diff-show-more-files-stats" @click.stop="loadMoreData">{{ showMoreMessage }}</a>
      </span>
    </li>
  </ol>
</template>

<script>
import {initTooltip} from '../modules/tippy.js';
import {doLoadMoreFiles} from '../features/repo-diff.js';

const {pageData} = window.config;

export default {
  name: 'DiffFileList',

  data: () => {
    return pageData.diffFileInfo;
  },

  watch: {
    fileListIsVisible(newValue) {
      if (newValue === true) {
        this.$nextTick(() => {
          for (const el of this.$refs.root.querySelectorAll('.tooltip')) {
            initTooltip(el);
          }
        });
      }
    }
  },

  mounted() {
    document.getElementById('show-file-list-btn').addEventListener('click', this.toggleFileList);
  },

  unmounted() {
    document.getElementById('show-file-list-btn').removeEventListener('click', this.toggleFileList);
  },

  methods: {
    toggleFileList() {
      this.fileListIsVisible = !this.fileListIsVisible;
    },
    diffTypeToString(pType) {
      const diffTypes = {
        1: 'add',
        2: 'modify',
        3: 'del',
        4: 'rename',
        5: 'copy',
      };
      return diffTypes[pType];
    },
    diffStatsWidth(adds, dels) {
      return `${adds / (adds + dels) * 100}%`;
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
