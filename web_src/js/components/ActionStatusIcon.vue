<!-- Keep in sync with templates/repo/icons/action_status.tmpl.
    action status accepted: success, skipped, waiting, blocked, running, failure, cancelled, cancelling, unknown.
-->
<script lang="ts" setup>
import {computed} from 'vue';
import SvgIcon from './SvgIcon.vue';
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
<<<<<<< HEAD
  <span :data-tooltip-content="localeStatus ?? status" v-if="status">
    <SvgIcon :name="circleFill ? 'octicon-check-circle-fill' : 'octicon-check'" class="text-green" :size="size" :class="className" v-if="status === 'success'"/>
    <SvgIcon name="octicon-skip" class="text-text-light" :size="size" :class="className" v-else-if="status === 'skipped'"/>
    <SvgIcon name="octicon-stop" class="text-text-light" :size="size" :class="className" v-else-if="status === 'cancelled'"/>
    <SvgIcon name="octicon-circle" class="text-text-light" :size="size" :class="className" v-else-if="status === 'waiting'"/>
    <SvgIcon name="octicon-blocked" class="text-yellow" :size="size" :class="className" v-else-if="status === 'blocked'"/>
    <SvgIcon name="gitea-running" class="text-yellow" :size="size" :class="'rotate-clockwise ' + className" v-else-if="status === 'running'"/>
    <SvgIcon name="octicon-stop" class="text-yellow" :size="size" :class="className" v-else-if="status === 'cancelling'"/>
    <SvgIcon :name="circleFill ? 'octicon-x-circle-fill' : 'octicon-x'" class="text-red" :size="size" :class="className" v-else/><!-- failure, unknown -->
=======
  <span class="flex-text-inline" :data-tooltip-content="localeStatus ?? status" v-if="status">
    <SvgIcon :name="icon.name" :class="iconClass" :size="size"/>
>>>>>>> origin/main
  </span>
</template>
