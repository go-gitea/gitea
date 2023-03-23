<template>
  <ol class="diff-detail-box diff-stats gt-m-0" ref="root" v-if="fileListIsVisible">
    <li v-for="file in files" :key="file.NameHash">
      <div class="gt-bold gt-df gt-ac pull-right">
        <span v-if="file.IsBin" class="gt-ml-1 gt-mr-3">{{ binaryFileMessage }}</span>
        {{ file.IsBin ? '' : file.Addition + file.Deletion }}
        <span v-if="!file.IsBin" class="diff-stats-bar gt-mx-3" :data-tooltip-content="statisticsMessage.replace('%d', (file.Addition + file.Deletion)).replace('%d', file.Addition).replace('%d', file.Deletion)">
          <div class="diff-stats-add-bar" :style="{ 'width': diffStatsWidth(file.Addition, file.Deletion) }" />
        </span>
      </div>
      <!-- todo finish all file status, now modify, add, delete and rename -->
      <span :class="['status', diffTypeToString(file.Type)]" :data-tooltip-content="diffTypeToString(file.Type)">&nbsp;</span>
      <a class="file gt-mono" :href="'#diff-' + file.NameHash">{{ file.Name }}</a>
    </li>
    <li v-if="isIncomplete" id="diff-too-many-files-stats" class="gt-pt-2">
      <span class="file gt-df gt-ac gt-sb">{{ tooManyFilesMessage }}
        <a :class="['ui', 'basic', 'tiny', 'button', isLoadingNewData === true ? 'disabled' : '']" id="diff-show-more-files-stats" @click.stop="loadMoreData">{{ showMoreMessage }}</a>
      </span>
    </li>
  </ol>
</template>

<script>
import {doLoadMoreFiles} from '../features/repo-diff.js';

const {pageData} = window.config;

export default {
  data: () => {
    return pageData.diffFileInfo;
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
