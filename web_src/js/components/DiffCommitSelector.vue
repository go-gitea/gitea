<template>
  <div
    class="ui dropdown basic button custom"
    @click.stop="menuVisible = !menuVisible" @keyup.enter="menuVisible = !menuVisible"
    :data-tooltip-content="locale.filter_changes_by_commit"
  >
    <svg-icon name="octicon-git-commit"/>
    <div class="menu left transition commit-selector-menu" :class="{visible: menuVisible}" v-if="menuVisible" v-cloak>
      <a class="vertical item gt-df gt-fc gt-gap-1" :href="issueLink + '/files' + queryParams">
        <div class="gt-ellipsis">{{ locale.show_all_commits }}</div>
        <div class="gt-ellipsis text light-2">{{ locale.stats_num_commits }}</div>
      </a>

      <div class="divider"/>
      <a v-if="lastReviewCommitSha != null" class="vertical item gt-df gt-fc gt-gap-1" @click="changesSinceLastReviewClick()">
        <div class="gt-ellipsis">{{ locale.show_changes_since_your_last_review }}</div>
        <div class="gt-ellipsis text light-2">{{ commitsSinceLastReview }} commits</div>
      </a>

      <template v-for="commit in commits" :key="commit.id">
        <div class="divider"/>
        <div class="vertical item gt-df gt-fr gt-gap-2" :class="{selected: commit.selected}" @click.exact="commitClicked(commit.id)" @click.shift.exact.stop.prevent="commitClickedShift(commit)">
          <div class="gt-f1 gt-df gt-fc gt-gap-1">
            <div class="gt-ellipsis commit-list-summary">
              {{ commit.summary }}
            </div>
            <div class="gt-ellipsis text light-2">
              {{ commit.committerOrAuthorName }}
              <span class="text right">
                <relative-time class="time-since" prefix="" :datetime="commit.time" data-tooltip-content data-tooltip-interactive="true">{{ commit.time }}</relative-time>
              </span>
            </div>
          </div>
          <div class="gt-mono">
            {{ commit.id }}
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script>
import {SvgIcon} from '../svg.js';

export default {
  components: {SvgIcon},
  data: () => {
    const commitInfo = window.config.pageData.commitInfo;
    return {
      menuVisible: false,
      locale: {},
      commits: [],
      queryParams: commitInfo.queryParams,
      issueLink: commitInfo.issueLink,
      hoverActivated: false,
      lastReviewCommitSha: null
    };
  },
  computed: {
    commitsSinceLastReview() {
      if (this.lastReviewCommitSha) {
        return this.commits.length - this.commits.findIndex((x) => x.id === this.lastReviewCommitSha) - 1;
      }
      return 0;
    }
  },
  mounted() {
    // fetch commit info
    this.fetchCommits();
  },
  methods: {
    async fetchCommits() {
        const resp = await fetch(`${this.issueLink}/commits/list`);
        const results = await resp.json();
        this.commits.push(...results.commits);
        this.lastReviewCommitSha = results.lastReviewCommitSha != '' ? results.lastReviewCommitSha : null;
        Object.assign(this.locale, results.locale);
    },
    /** Called when user clicks on since last review */
    changesSinceLastReviewClick() {
      window.location = `${this.issueLink}/files/${this.lastReviewCommitSha}..${this.commits.at(-1)}${this.queryParams}`;
    },
    /** Clicking on a single commit opens this specific commit */
    commitClicked(commitId) {
      window.location = `${this.issueLink}/commits/${commitId}${this.queryParams}`;
    },
    /**
     * When a commit is clicked with shift this enables the range
     * selection. Second click (with shift) defines the end of the
     * range. This opens the diff of this range
     * Exception: first commit is the first commit of this PR. Then
     * the diff from beginning of PR up to the second clicked commit is
     * opened
     */
    commitClickedShift(commit) {
      this.hoverActivated = !this.hoverActivated;
      commit.selected = true;
      // Second click -> determine our range and open links accordingly
      if (!this.hoverActivated) {
        // find all selected commits and generate a link
        if (this.commits[0].selected) {
          // first commit is selected - generate a short url with only target sha
          const lastCommit = this.commits.findLast((x) => x.selected);
          window.location = `${this.issueLink}/files/${lastCommit.id}${this.queryParams}`;
        } else {
          const start = this.commits[this.commits.findIndex((x) => x.selected) - 1].id;
          const end = this.commits.findLast((x) => x.selected).id;
          window.location = `${this.issueLink}/files/${start}..${end}${this.queryParams}`;
        }
      }
    },
  }
};
</script>
<style scoped>
  .selected {
    background-color: var(--color-secondary-active) !important;
  }
</style>
