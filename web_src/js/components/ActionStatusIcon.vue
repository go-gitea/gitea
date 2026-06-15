<!-- Keep in sync with templates/repo/icons/action_status.tmpl.
    action status accepted: success, skipped, waiting, blocked, running, failure, cancelled, cancelling, unknown.
-->
<script lang="ts" setup>
import {computed} from 'vue';
import {SvgIcon} from '../svg.ts';
import {getActionStatusIcon, type ActionStatusIconVariant} from '../modules/action-status-icon.ts';

const props = withDefaults(defineProps<{
  status: 'success' | 'skipped' | 'waiting' | 'blocked' | 'running' | 'failure' | 'cancelled' | 'cancelling' | 'unknown',
  size?: number,
  className?: string,
  localeStatus?: string,
  iconVariant?: ActionStatusIconVariant,
}>(), {
  size: 16,
  className: '',
  localeStatus: undefined,
  iconVariant: '',
});

const icon = computed(() => getActionStatusIcon(props.status, props.iconVariant));
const iconClass = computed(() => {
  const classes = [icon.value.colorClass, props.className];
  if (props.status === 'running') classes.push('rotate-clockwise');
  return classes.filter(Boolean).join(' ');
});
</script>

<template>
  <span :data-tooltip-content="localeStatus ?? status" v-if="status">
    <SvgIcon :name="icon.name" :class="iconClass" :size="size"/>
  </span>
</template>
