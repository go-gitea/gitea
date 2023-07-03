<template>
  <div
    class="ui jump dropdown basic button custom"
    @click.stop="menuVisible = !menuVisible" @keyup.enter="menuVisible = !menuVisible"
    :data-tooltip-content="locale.filter_changes_by_commit">
    <svg-icon name="octicon-git-commit"/>
    <div class="menu left transition commit-selector-menu" :class="{visible: menuVisible}" v-if="menuVisible" v-cloak>
      <a class="vertical item gt-df gt-fc gt-gap-1" :href="issueLink + '/files' + queryParams">
        <div class="gt-ellipsis">{{ locale.show_all_commits }}</div>
        <div class="gt-ellipsis text light-2">{{ locale.stats_num_commits }}</div>
      </a>

      <template v-for="commit in commits" :key="commit.ID">
        <div class="divider"/>
        <div class="vertical item gt-df gt-fr gt-gap-2" :class="{hovered: commit.Hovered}" @click.exact="commitClicked(commit.ID)" @click.shift.exact.stop.prevent="commitClickedShift(commit)">
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
          <div class="gt-mono">{{ commit.ID }}</div>
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
    commitClicked(commitId) {
      window.location = `${this.issueLink}/files/${commitId}${this.queryParams}`;
    },
    commitClickedShift(commit) {
      this.hoverActivated = !this.hoverActivated;
      commit.Hovered = true;
      if (!this.hoverActivated) {
        // find all hovered commits and generate a link
        if (this.commits[0].Hovered) {
          // first commit is hovered - generate a short url with only target sha
          const lastCommit = this.commits.findLast((x) => x.Hovered);
          window.location = `${this.issueLink}/files/${lastCommit.ID}${this.queryParams}`;
        } else {
          const start = this.commits[this.commits.findIndex((x) => x.Hovered) - 1].ID;
          const end = this.commits.findLast((x) => x.Hovered).ID;
          window.location = `${this.issueLink}/files/${start}..${end}${this.queryParams}`;
        }
      }
    },
  }
};
</script>

<style scoped>
  .hovered {
    background-color: purple !important;
  }
</style>
