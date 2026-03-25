<!-- This vue should be kept the same as templates/repo/actions/status.tmpl
    Please also update the template file above if this vue is modified.
    action status accepted: success, skipped, waiting, blocked, running, failure, cancelled, unknown
-->
<script lang="ts" setup>
import {SvgIcon} from '../svg.ts';

withDefaults(defineProps<{
  status: 'success' | 'skipped' | 'waiting' | 'blocked' | 'running' | 'failure' | 'cancelled' | 'unknown',
  size?: number,
  className?: string,
  localeStatus?: string,
}>(), {
  size: 16,
  className: '',
  localeStatus: undefined,
});
</script>

<template>
  <span :data-tooltip-content="localeStatus ?? status" v-if="status">
    <SvgIcon name="octicon-check-circle-fill" class="tw-text-green" :size="size" :class="className" v-if="status === 'success'"/>
    <SvgIcon name="octicon-skip" class="tw-text-text-light" :size="size" :class="className" v-else-if="status === 'skipped'"/>
    <SvgIcon name="octicon-stop" class="tw-text-text-light" :size="size" :class="className" v-else-if="status === 'cancelled'"/>
    <SvgIcon name="octicon-circle" class="tw-text-text-light" :size="size" :class="className" v-else-if="status === 'waiting'"/>
    <SvgIcon name="octicon-blocked" class="tw-text-yellow" :size="size" :class="className" v-else-if="status === 'blocked'"/>
    <SvgIcon name="gitea-running" class="tw-text-yellow" :size="size" :class="'rotate-clockwise ' + className" v-else-if="status === 'running'"/>
    <SvgIcon name="octicon-x-circle-fill" class="tw-text-red" :size="size" v-else/><!-- failure, unknown -->
  </span>
</template>
