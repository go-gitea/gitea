<script>
import {initComponent} from '../../init.js';
import {SvgIcon} from '../../svg.js';
import {POST} from '../../modules/fetch.js';
import {showErrorToast} from '../../modules/toast.js';
import { ref } from 'vue';

const sfc = {
  name: 'IssueSubscribe',
  components: {SvgIcon},
  props: {
    isWatchingReadOnly: {
      type: Boolean,
      default: false,
    },
    watchLink: {
      type: String,
      default: false,
    },
    locale: {
      type: Object,
      default: () => {},
    }
  },
  data() {
    return {
      isLoading: false,
      isWatching: false,
    };
  },
  setup(props) {
    const isWatching = ref(props.isWatchingReadOnly);
    return {
      isWatching,
    };
  },
  methods: {
    async toggleSubscribe() {
      if (this.isLoading) return;

      this.isLoading = true;
      try {
        const resp = await POST(`${this.watchLink}?watch=${!this.isWatching}`);
        console.info(resp.status, resp.status === 200);
        if (resp.status !== 200) {
          showErrorToast(`Update watching status return: ${resp.status}`);
          return;
        }

        this.isWatching = !this.isWatching;
      } catch (e) {
        showErrorToast(`Network error when fetching ${this.mode}, error: ${e}`);
      } finally {
        this.isLoading = false;
      }
    },
  }
};

export default sfc;

export function initIssueSubsribe() {
  initComponent('issue-subscribe', sfc);
}
</script>
<template>
  <div class="ui watching">
    <span class="text"><strong>{{ locale.notifications }}</strong></span>
    <div class="gt-mt-3">
      <button class="fluid ui button" @click="toggleSubscribe">
        <SvgIcon :name="isWatching?'octicon-mute':'octicon-unmute'" class="text white" :size="16" class-name="gt-mr-3"/>
        {{ isWatching?locale.unsubscribe:locale.subscribe }}
      </button>
    </div>
  </div>
</template>
