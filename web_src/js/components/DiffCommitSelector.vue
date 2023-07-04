<template>
  <div
    class="ui jump dropdown basic button custom"
    @click.stop="menuVisible = !menuVisible" @keyup.enter="menuVisible = !menuVisible"
    :data-tooltip-content="locale.filter_changes_by_commit"
  >
    <svg-icon name="octicon-git-commit"/>
    <div class="menu left transition commit-selector-menu" :class="{visible: menuVisible}" v-if="menuVisible" v-cloak>
      <a class="vertical item gt-df gt-fc gt-gap-1" :href="issueLink + '/files' + queryParams">
        <div class="gt-ellipsis">{{ locale.show_all_commits }}</div>
        <div class="gt-ellipsis text light-2">{{ locale.stats_num_commits }}</div>
      </a>

      <template v-for="commit in commits" :key="commit.ID">
        <div class="divider"/>
        <div class="vertical item gt-df gt-fr gt-gap-2" :class="{selected: commit.Selected}" @click.exact="commitClicked(commit.ID)" @click.shift.exact.stop.prevent="commitClickedShift(commit)">
          <div class="gt-f1 gt-df gt-fc gt-gap-1">
            <div class="gt-ellipsis commit-list-summary">
              {{ commit.Summary }}
            </div>
            <div class="gt-ellipsis text light-2">
              {{ commit.CommitterOrAuthorName }}
              <span class="text right">
                <relative-time class="time-since" prefix="" :datetime="commit.Time" data-tooltip-content data-tooltip-interactive="true">{{ commit.Time }}</relative-time>
              </span>
            </div>
          </div>
          <div class="gt-mono">
            {{ commit.ID }}
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
      locale: window.config.pageData.commitInfo.locale,
      commits: window.config.pageData.commitInfo.commits.reverse(),
      queryParams: window.config.pageData.commitInfo.queryParams,
      issueLink: window.config.pageData.commitInfo.issueLink,
      hoverActivated: false
    };
  },
  methods: {
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
      commit.Selected = true;
      // Second click -> determine our range and open links accordingly
      if (!this.hoverActivated) {
        // find all selected commits and generate a link
        if (this.commits[0].Selected) {
          // first commit is selected - generate a short url with only target sha
          const lastCommit = this.commits.findLast((x) => x.Selected);
          window.location = `${this.issueLink}/files/${lastCommit.ID}${this.queryParams}`;
        } else {
          const start = this.commits[this.commits.findIndex((x) => x.Selected) - 1].ID;
          const end = this.commits.findLast((x) => x.Selected).ID;
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
