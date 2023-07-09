<template>
  <div
    class="ui dropdown basic button custom"
    @click.stop="toggleMenu()" @keyup.enter="toggleMenu()"
    :data-tooltip-content="locale.filter_changes_by_commit"
  >
    <svg-icon name="octicon-git-commit"/>
    <div class="menu left transition commit-selector-menu" :class="{visible: menuVisible}" v-if="menuVisible" v-cloak>
      <div class="vertical item gt-df gt-fc gt-gap-1 gt-border-secondary-top" @click="showAllChanges()">
        <div class="gt-ellipsis">
          {{ locale.show_all_commits }}
        </div>
        <div class="gt-ellipsis text light-2">
          {{ locale.stats_num_commits }}
        </div>
      </div>

      <!-- only show the show changes since last review if there is a review AND we are commits ahead of the last review -->
      <div v-if="lastReviewCommitSha != null && commitsSinceLastReview > 0" class="vertical item gt-df gt-fc gt-gap-1 gt-border-secondary-top" @click="changesSinceLastReviewClick()">
        <div class="gt-ellipsis">
          {{ locale.show_changes_since_your_last_review }}
        </div>
        <div class="gt-ellipsis text light-2">
          {{ commitsSinceLastReview }} commits
        </div>
      </div>

      <span class="info gt-border-secondary-top">{{ locale.select_commit_hold_shift_for_range }}</span>

      <template v-for="commit in commits" :key="commit.id">
        <div class="vertical item gt-df gt-fr gt-gap-2 gt-border-secondary-top" :class="{selected: commit.selected}" @click.exact="commitClicked(commit.id)" @click.shift.exact.stop.prevent="commitClickedShift(commit)">
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
    return {
      menuVisible: false,
      locale: {},
      commits: [],
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
    },
    queryParams() {
      return this.$el.parentNode.getAttribute('data-queryparams');
    },
    issueLink() {
      return this.$el.parentNode.getAttribute('data-issuelink');
    }
  },
  methods: {
    /** Opens our menu, loads commits before opening */
    async toggleMenu() {
      // load our commits when the menu is not yet visible (it'll be toggled after loading)
      // and we got no commits
      if (this.commits.length === 0 && this.menuVisible === false) {
        await this.fetchCommits();
      }
      this.menuVisible = !this.menuVisible;
    },
    /** Load the commits to show in this dropdown */
    async fetchCommits() {
      const resp = await fetch(`${this.issueLink}/commits/list`);
      const results = await resp.json();
      this.commits.push(...results.commits);
      this.commits.reverse();
      this.lastReviewCommitSha = results.lastReviewCommitSha !== '' ? results.lastReviewCommitSha : null;
      Object.assign(this.locale, results.locale);
    },
    showAllChanges() {
      window.location = `${this.issueLink}/files${this.queryParams}`;
    },
    /** Called when user clicks on since last review */
    changesSinceLastReviewClick() {
      window.location = `${this.issueLink}/files/${this.lastReviewCommitSha}..${this.commits.at(-1).id}${this.queryParams}`;
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

  .info {
    display: inline-block;
    padding: 7px 14px !important;
    line-height: 1.4;
    width: 100%;
  }
</style>
