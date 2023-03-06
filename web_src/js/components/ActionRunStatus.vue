<template>
  <SvgIcon name="octicon-check-circle-fill" class="green" :size="size" v-if="status === 'success'"/>
  <SvgIcon name="octicon-skip" class="ui text grey" :size="size" v-else-if="status === 'skipped'"/>
  <SvgIcon name="octicon-clock" class="ui text yellow" :size="size" v-else-if="status === 'waiting'"/>
  <SvgIcon name="octicon-blocked" class="ui text yellow" :size="size" v-else-if="status === 'blocked'"/>
  <SvgIcon name="octicon-meter" class="ui text yellow" :size="size" class-name="job-status-rotate" v-else-if="status === 'running'"/>
  <SvgIcon name="octicon-x-circle-fill" class="red" :size="size" v-else/>
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
    }
  },
};
</script>
