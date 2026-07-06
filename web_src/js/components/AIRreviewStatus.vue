<script lang="ts" setup>
import {ref, onMounted, computed} from 'vue';
import {SvgIcon} from '../svg.ts';

const props = defineProps<{
  statusUrl: string;
}>();

type ReviewStatus = 'pending' | 'running' | 'completed' | 'issues_found' | 'error' | '';

const status = ref<ReviewStatus>('');
const issueCount = ref(0);
const loading = ref(true);

const iconName = computed(() => {
  switch (status.value) {
    case 'pending': return 'octicon-clock';
    case 'running': return 'gitea-running';
    case 'completed': return 'octicon-check';
    case 'issues_found': return 'octicon-stop';
    case 'error': return 'octicon-x';
    default: return '';
  }
});

const iconClass = computed(() => {
  const classes = [];
  switch (status.value) {
    case 'running': classes.push('rotate-clockwise');
    case 'completed': classes.push('tw-text-green');
    case 'issues_found': classes.push('tw-text-yellow-700');
    case 'error': classes.push('tw-text-red');
    case 'pending': classes.push('tw-text-gray');
  }
  return classes.join(' ');
});

const label = computed(() => {
  switch (status.value) {
    case 'pending': return 'AI Review pending';
    case 'running': return 'AI Review in progress';
    case 'completed': return 'AI Review passed';
    case 'issues_found': return `AI Review: ${issueCount.value} issue${issueCount.value !== 1 ? 's' : ''} found`;
    case 'error': return 'AI Review failed';
    default: return '';
  }
});

async function fetchStatus() {
  try {
    const resp = await fetch(props.statusUrl);
    if (!resp.ok) {
      status.value = 'error';
      return;
    }
    const data = await resp.json();
    status.value = data.status;
    issueCount.value = data.issue_count ?? 0;
  } catch {
    status.value = 'error';
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  fetchStatus();
});
</script>

<template>
  <div v-if="status && !loading" class="tw-flex tw-items-center tw-gap-1 tw-text-13">
    <SvgIcon :name="iconName" :class="iconClass" :size="14"/>
    <span>{{ label }}</span>
  </div>
</template>
