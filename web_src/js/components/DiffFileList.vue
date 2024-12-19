<script lang="ts" setup>
import {onMounted, onUnmounted} from 'vue';
import {loadMoreFiles} from '../features/repo-diff.ts';
import {diffTreeStore} from '../modules/stores.ts';

const store = diffTreeStore();

onMounted(() => {
  document.querySelector('#show-file-list-btn').addEventListener('click', toggleFileList);
});

onUnmounted(() => {
  document.querySelector('#show-file-list-btn').removeEventListener('click', toggleFileList);
});

function toggleFileList() {
  store.fileListIsVisible = !store.fileListIsVisible;
}

function diffTypeToString(pType) {
  const diffTypes = {
    1: 'add',
    2: 'modify',
    3: 'del',
    4: 'rename',
    5: 'copy',
  };
  return diffTypes[pType];
}

function diffStatsWidth(adds, dels) {
  return `${adds / (adds + dels) * 100}%`;
}

function loadMoreData() {
  loadMoreFiles(store.linkLoadMore);
}
</script>

<template>
  <ol class="diff-stats tw-m-0" ref="root" v-if="store.fileListIsVisible">
    <li v-for="file in store.files" :key="file.NameHash">
      <div class="tw-font-semibold tw-flex tw-items-center pull-right">
        <span v-if="file.IsBin" class="tw-ml-0.5 tw-mr-2">{{ store.binaryFileMessage }}</span>
        {{ file.IsBin ? '' : file.Addition + file.Deletion }}
        <span v-if="!file.IsBin" class="diff-stats-bar tw-mx-2" :data-tooltip-content="store.statisticsMessage.replace('%d', (file.Addition + file.Deletion)).replace('%d', file.Addition).replace('%d', file.Deletion)">
          <div class="diff-stats-add-bar" :style="{ 'width': diffStatsWidth(file.Addition, file.Deletion) }"/>
        </span>
      </div>
      <!-- todo finish all file status, now modify, add, delete and rename -->
      <span :class="['status', diffTypeToString(file.Type)]" :data-tooltip-content="diffTypeToString(file.Type)">&nbsp;</span>
      <a class="file tw-font-mono" :href="'#diff-' + file.NameHash">{{ file.Name }}</a>
    </li>
    <li v-if="store.isIncomplete" class="tw-pt-1">
      <span class="file tw-flex tw-items-center tw-justify-between">{{ store.tooManyFilesMessage }}
        <a :class="['ui', 'basic', 'tiny', 'button', store.isLoadingNewData ? 'disabled' : '']" @click.stop="loadMoreData">{{ store.showMoreMessage }}</a>
      </span>
    </li>
  </ol>
</template>
