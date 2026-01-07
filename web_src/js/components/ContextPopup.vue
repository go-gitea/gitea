<script lang="ts" setup>
import {SvgIcon} from '../svg.ts';
import {GET} from '../modules/fetch.ts';
import {getIssueColor, getIssueIcon} from '../features/issue.ts';
import {computed, onMounted, shallowRef} from 'vue';
import type {Issue} from '../types.ts';

const props = defineProps<{
  repoLink: string,
  loadIssueInfoUrl: string,
}>();

const loading = shallowRef(false);
const issue = shallowRef<Issue | null>(null);
const renderedLabels = shallowRef('');
const errorMessage = shallowRef('');

const createdAt = computed(() => {
  if (!issue?.value) return '';
  return new Date(issue.value.created_at).toLocaleDateString(undefined, {year: 'numeric', month: 'short', day: 'numeric'});
});

const body = computed(() => {
  if (!issue?.value) return '';
  const body = issue.value.body.replace(/\n+/g, ' ');
  return body.length > 85 ? `${body.substring(0, 85)}…` : body;
});

onMounted(async () => {
  loading.value = true;
  errorMessage.value = '';
  try {
    const resp = await GET(props.loadIssueInfoUrl);
    if (!resp.ok) {
      errorMessage.value = resp.status ? resp.statusText : 'Unknown network error';
      return;
    }
    const respJson = await resp.json();
    issue.value = respJson.convertedIssue;
    renderedLabels.value = respJson.renderedLabels;
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div class="tw-p-4">
    <div v-if="loading" class="tw-h-12 tw-w-12 is-loading"/>
    <div v-else-if="issue" class="tw-flex tw-flex-col tw-gap-2">
      <div class="tw-text-12">
        <a :href="repoLink" class="muted">{{ issue.repository.full_name }}</a>
        on {{ createdAt }}
      </div>
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
    <div v-else>
      {{ errorMessage }}
    </div>
  </div>
</template>
