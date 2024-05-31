<script>
import {SvgIcon} from '../svg.js';

export default {
  components: {SvgIcon},
  props: {
    issue: {
      type: Object,
      default: null,
    },
    labelsHtml: {
      type: String,
      default: '',
    },
    repoUrl: {
      type: String,
      default: '',
    },
  },
  computed: {
    createdAt() {
      return new Date(this.issue.created_at).toLocaleDateString(undefined, {year: 'numeric', month: 'short', day: 'numeric'});
    },

    body() {
      const body = this.issue.body.replace(/\n+/g, ' ');
      if (body.length > 85) {
        return `${body.substring(0, 85)}…`;
      }
      return body;
    },

    icon() {
      if (this.issue.pull_request !== null) {
        if (this.issue.state === 'open') {
          if (this.issue.pull_request.draft === true) {
            return 'octicon-git-pull-request-draft'; // WIP PR
          }
          return 'octicon-git-pull-request'; // Open PR
        } else if (this.issue.pull_request.merged === true) {
          return 'octicon-git-merge'; // Merged PR
        }
        return 'octicon-git-pull-request'; // Closed PR
      } else if (this.issue.state === 'open') {
        return 'octicon-issue-opened'; // Open Issue
      }
      return 'octicon-issue-closed'; // Closed Issue
    },

    color() {
      if (this.issue.pull_request !== null) {
        if (this.issue.pull_request.draft === true) {
          return 'grey'; // WIP PR
        } else if (this.issue.pull_request.merged === true) {
          return 'purple'; // Merged PR
        }
      }
      if (this.issue.state === 'open') {
        return 'green'; // Open Issue
      }
      return 'red'; // Closed Issue
    },
  },
};
</script>
<template>
  <div class="tw-p-3 tw-flex tw-flex-col tw-gap-2">
    <div class="tw-text-12">
      <a class="muted" :href="repoUrl">{{ issue.repository.full_name }}</a> on {{ createdAt }}
    </div>
    <div class="flex-text-block tw-gap-2">
      <svg-icon :name="icon" :class="['text', color]"/>
      <span class="issue-title tw-font-semibold tw-break-anywhere">
        {{ issue.title }}
        <span class="index">#{{ issue.number }}</span>
      </span>
    </div>
    <div v-if="body">{{ body }}</div>
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div v-if="issue.labels.length" v-html="labelsHtml"/>
  </div>
</template>
