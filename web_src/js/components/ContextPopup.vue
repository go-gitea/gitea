<template>
  <div ref="root">
    <div v-if="loading" class="ui active centered inline loader"/>
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

<script>
import $ from 'jquery';
import {SvgIcon} from '../svg.js';
import {useLightTextOnBackground, hexToRGBColor} from '../utils/color.js';

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
      if (this.issue.state === 'open') {
        return 'green';
      } else if (this.issue.pull_request !== null && this.issue.pull_request.merged === true) {
        return 'purple';
      }
      return 'red';
    },

    labels() {
      return this.issue.labels.map((label) => {
        let textColor;
        const [r, g, b] = hexToRGBColor(label.color);
        if (useLightTextOnBackground(r, g, b)) {
          textColor = '#eeeeee';
        } else {
          textColor = '#111111';
        }
        return {name: label.name, color: `#${label.color}`, textColor};
      });
    }
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
    load(data) {
      this.loading = true;
      this.i18nErrorMessage = null;
      $.get(`${appSubUrl}/${data.owner}/${data.repo}/issues/${data.index}/info`).done((issue) => {
        this.issue = issue;
      }).fail((jqXHR) => {
        if (jqXHR.responseJSON && jqXHR.responseJSON.message) {
          this.i18nErrorMessage = jqXHR.responseJSON.message;
        } else {
          this.i18nErrorMessage = i18n.network_error;
        }
      }).always(() => {
        this.loading = false;
      });
    }
  }
};
</script>
