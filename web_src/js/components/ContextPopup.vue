<script lang="ts" setup>
import {SvgIcon} from '../svg.ts';
import {GET} from '../modules/fetch.ts';
import {getIssueColor, getIssueIcon} from '../features/issue.ts';
import {computed, onMounted, ref} from 'vue';
import type {IssuePathInfo} from '../types.ts';

const {appSubUrl, i18n} = window.config;

const loading = ref(false);
const issue = ref(null);
const renderedLabels = ref('');
const i18nErrorOccurred = i18n.error_occurred;
const i18nErrorMessage = ref(null);

const createdAt = computed(() => new Date(issue.value.created_at).toLocaleDateString(undefined, {year: 'numeric', month: 'short', day: 'numeric'}));
const body = computed(() => {
  const body = issue.value.body.replace(/\n+/g, ' ');
  if (body.length > 85) {
    return `${body.substring(0, 85)}…`;
  }
  return body;
});

const root = ref<HTMLElement | null>(null);

onMounted(() => {
  root.value.addEventListener('ce-load-context-popup', (e: CustomEventInit<IssuePathInfo>) => {
    if (!loading.value && issue.value === null) {
      load(e.detail);
    }
  });
});

async function load(issuePathInfo: IssuePathInfo) {
  loading.value = true;
  i18nErrorMessage.value = null;

  try {
    const response = await GET(`${appSubUrl}/${issuePathInfo.ownerName}/${issuePathInfo.repoName}/issues/${issuePathInfo.indexString}/info`); // backend: GetIssueInfo
    const respJson = await response.json();
    if (!response.ok) {
      i18nErrorMessage.value = respJson.message ?? i18n.network_error;
      return;
    }
    issue.value = respJson.convertedIssue;
    renderedLabels.value = respJson.renderedLabels;
  } catch {
    i18nErrorMessage.value = i18n.network_error;
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <div ref="root">
    <div v-if="loading" class="tw-h-12 tw-w-12 is-loading"/>
    <div v-if="!loading && issue !== null" class="tw-flex tw-flex-col tw-gap-2">
      <div class="tw-text-12">{{ issue.repository.full_name }} on {{ createdAt }}</div>
      <div class="flex-text-block">
        <svg-icon :name="getIssueIcon(issue)" :class="['text', getIssueColor(issue)]"/>
        <span class="issue-title tw-font-semibold tw-break-anywhere">
          {{ issue.title }}
          <span class="index">#{{ issue.number }}</span>
        </span>
      </div>
      <div v-if="body">{{ body }}</div>
      <!-- eslint-disable-next-line vue/no-v-html -->
      <div v-if="issue.labels.length" v-html="renderedLabels"/>
    </div>
    <div class="tw-flex tw-flex-col tw-gap-2" v-if="!loading && issue === null">
      <div class="tw-text-12">{{ i18nErrorOccurred }}</div>
      <div>{{ i18nErrorMessage }}</div>
    </div>
  </div>
</template>
