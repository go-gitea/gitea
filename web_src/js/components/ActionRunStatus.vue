<!-- This vue should be kept the same as templates/repo/actions/status.tmpl
    Please also update the template file above if this vue is modified.
    action status accepted: success, skipped, waiting, blocked, running, failure, cancelled, unknown
-->
<template>
  <span class="gt-df gt-ac" :data-tooltip-content="localeStatus" v-if="status">
    <SvgIcon name="octicon-check-circle-fill" class="text green" :size="size" :class-name="className" v-if="status === 'success'"/>
    <SvgIcon name="octicon-skip" class="text grey" :size="size" :class-name="className" v-else-if="status === 'skipped'"/>
    <SvgIcon name="octicon-clock" class="text yellow" :size="size" :class-name="className" v-else-if="status === 'waiting'"/>
    <SvgIcon name="octicon-blocked" class="text yellow" :size="size" :class-name="className" v-else-if="status === 'blocked'"/>
    <SvgIcon name="octicon-meter" class="text yellow" :size="size" :class-name="'job-status-rotate ' + className" v-else-if="status === 'running'"/>
    <SvgIcon name="octicon-x-circle-fill" class="text red" :size="size" v-else-if="['failure', 'cancelled', 'unknown'].includes(status)"/>
  </span>
</template>

<script>
import {SvgIcon} from '../svg.js';

export default {
  components: {SvgIcon},
  props: {
    status: {
      type: String,
      required: true
    },
    size: {
      type: Number,
      default: 16
    },
    className: {
      type: String,
      default: ''
    },
    localeStatus: {
      type: String,
      default: ''
    }
  },
};
</script>
