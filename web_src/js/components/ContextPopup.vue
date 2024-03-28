<script>
import {SvgIcon} from '../svg.js';
import {useLightTextOnBackground} from '../utils/color.js';
import tinycolor from 'tinycolor2';
import {GET} from '../modules/fetch.js';

const {appSubUrl, i18n} = window.config;

export default {
  components: {SvgIcon},
  data: () => ({
    loading: false,
    issue: null,
    i18nErrorOccurred: i18n.error_occurred,
    i18nErrorMessage: null,
  }),
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

    labels() {
      return this.issue.labels.map((label) => {
        let textColor;
        const {r, g, b} = tinycolor(label.color).toRgb();
        if (useLightTextOnBackground(r, g, b)) {
          textColor = '#eeeeee';
        } else {
          textColor = '#111111';
        }
        return {name: label.name, color: `#${label.color}`, textColor};
      });
    },
  },
  mounted() {
    this.$refs.root.addEventListener('ce-load-context-popup', (e) => {
      const data = e.detail;
      if (!this.loading && this.issue === null) {
        this.load(data);
      }
    });
  },
  methods: {
    async load(data) {
      this.loading = true;
      this.i18nErrorMessage = null;

      try {
        const response = await GET(`${appSubUrl}/${data.owner}/${data.repo}/issues/${data.index}/info`);
        const respJson = await response.json();
        if (!response.ok) {
          this.i18nErrorMessage = respJson.message ?? i18n.network_error;
          return;
        }
        this.issue = respJson;
      } catch {
        this.i18nErrorMessage = i18n.network_error;
      } finally {
        this.loading = false;
      }
    },
  },
};
</script>
<template>
  <div ref="root">
    <div v-if="loading" class="tw-h-12 tw-w-12 is-loading"/>
    <div v-if="!loading && issue !== null">
      <p><small>{{ issue.repository.full_name }} on {{ createdAt }}</small></p>
      <p><svg-icon :name="icon" :class="['text', color]"/> <strong>{{ issue.title }}</strong> #{{ issue.number }}</p>
      <p>{{ body }}</p>
      <div>
        <div
          v-for="label in labels"
          :key="label.name"
          class="ui label"
          :style="{ color: label.textColor, backgroundColor: label.color }"
        >
          {{ label.name }}
        </div>
      </div>
    </div>
    <div v-if="!loading && issue === null">
      <p><small>{{ i18nErrorOccurred }}</small></p>
      <p>{{ i18nErrorMessage }}</p>
    </div>
  </div>
</template>
