<script>
import {SvgIcon} from '../svg.js';
import {GET} from '../modules/fetch.js';

export default {
  components: {SvgIcon},
  data: () => {
    const el = document.querySelector('#diff-commit-select');
    return {
      menuVisible: false,
      isLoading: false,
      locale: {
        filter_changes_by_commit: el.getAttribute('data-filter_changes_by_commit'),
      },
      commits: [],
      hoverActivated: false,
      lastReviewCommitSha: null,
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
    },
  },
  mounted() {
    document.body.addEventListener('click', this.onBodyClick);
    this.$el.addEventListener('keydown', this.onKeyDown);
    this.$el.addEventListener('keyup', this.onKeyUp);
  },
  unmounted() {
    document.body.removeEventListener('click', this.onBodyClick);
    this.$el.removeEventListener('keydown', this.onKeyDown);
    this.$el.removeEventListener('keyup', this.onKeyUp);
  },
  methods: {
    onBodyClick(event) {
      // close this menu on click outside of this element when the dropdown is currently visible opened
      if (this.$el.contains(event.target)) return;
      if (this.menuVisible) {
        this.toggleMenu();
      }
    },
    onKeyDown(event) {
      if (!this.menuVisible) return;
      const item = document.activeElement;
      if (!this.$el.contains(item)) return;
      switch (event.key) {
        case 'ArrowDown': // select next element
          event.preventDefault();
          this.focusElem(item.nextElementSibling, item);
          break;
        case 'ArrowUp': // select previous element
          event.preventDefault();
          this.focusElem(item.previousElementSibling, item);
          break;
        case 'Escape': // close menu
          event.preventDefault();
          item.tabIndex = -1;
          this.toggleMenu();
          break;
      }
    },
    onKeyUp(event) {
      if (!this.menuVisible) return;
      const item = document.activeElement;
      if (!this.$el.contains(item)) return;
      if (event.key === 'Shift' && this.hoverActivated) {
        // shift is not pressed anymore -> deactivate hovering and reset hovered and selected
        this.hoverActivated = false;
        for (const commit of this.commits) {
          commit.hovered = false;
          commit.selected = false;
        }
      }
    },
    highlight(commit) {
      if (!this.hoverActivated) return;
      const indexSelected = this.commits.findIndex((x) => x.selected);
      const indexCurrentElem = this.commits.findIndex((x) => x.id === commit.id);
      for (const [idx, commit] of this.commits.entries()) {
        commit.hovered = Math.min(indexSelected, indexCurrentElem) <= idx && idx <= Math.max(indexSelected, indexCurrentElem);
      }
    },
    /** Focus given element */
    focusElem(elem, prevElem) {
      if (elem) {
        elem.tabIndex = 0;
        if (prevElem) prevElem.tabIndex = -1;
        elem.focus();
      }
    },
    /** Opens our menu, loads commits before opening */
    async toggleMenu() {
      this.menuVisible = !this.menuVisible;
      // load our commits when the menu is not yet visible (it'll be toggled after loading)
      // and we got no commits
      if (!this.commits.length && this.menuVisible && !this.isLoading) {
        this.isLoading = true;
        try {
          await this.fetchCommits();
        } finally {
          this.isLoading = false;
        }
      }
      // set correct tabindex to allow easier navigation
      this.$nextTick(() => {
        const expandBtn = this.$el.querySelector('#diff-commit-list-expand');
        const showAllChanges = this.$el.querySelector('#diff-commit-list-show-all');
        if (this.menuVisible) {
          this.focusElem(showAllChanges, expandBtn);
        } else {
          this.focusElem(expandBtn, showAllChanges);
        }
      });
    },
    /** Load the commits to show in this dropdown */
    async fetchCommits() {
      const resp = await GET(`${this.issueLink}/commits/list`);
      const results = await resp.json();
      this.commits.push(...results.commits.map((x) => {
        x.hovered = false;
        return x;
      }));
      this.commits.reverse();
      this.lastReviewCommitSha = results.last_review_commit_sha || null;
      if (this.lastReviewCommitSha && this.commits.findIndex((x) => x.id === this.lastReviewCommitSha) === -1) {
        // the lastReviewCommit is not available (probably due to a force push)
        // reset the last review commit sha
        this.lastReviewCommitSha = null;
      }
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
    commitClicked(commitId, newWindow = false) {
      const url = `${this.issueLink}/commits/${commitId}${this.queryParams}`;
      if (newWindow) {
        window.open(url);
      } else {
        window.location = url;
      }
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
          const lastCommitIdx = this.commits.findLastIndex((x) => x.selected);
          if (lastCommitIdx === this.commits.length - 1) {
            // user selected all commits - just show the normal diff page
            window.location = `${this.issueLink}/files${this.queryParams}`;
          } else {
            window.location = `${this.issueLink}/files/${this.commits[lastCommitIdx].id}${this.queryParams}`;
          }
        } else {
          const start = this.commits[this.commits.findIndex((x) => x.selected) - 1].id;
          const end = this.commits.findLast((x) => x.selected).id;
          window.location = `${this.issueLink}/files/${start}..${end}${this.queryParams}`;
        }
      }
    },
  },
};
</script>
<template>
  <div class="ui scrolling dropdown custom">
    <button
      class="ui basic button"
      id="diff-commit-list-expand"
      @click.stop="toggleMenu()"
      :data-tooltip-content="locale.filter_changes_by_commit"
      aria-haspopup="true"
      aria-controls="diff-commit-selector-menu"
      :aria-label="locale.filter_changes_by_commit"
      aria-activedescendant="diff-commit-list-show-all"
    >
      <svg-icon name="octicon-git-commit"/>
    </button>
    <div class="left menu" id="diff-commit-selector-menu" :class="{visible: menuVisible}" v-show="menuVisible" v-cloak :aria-expanded="menuVisible ? 'true': 'false'">
      <div class="loading-indicator is-loading" v-if="isLoading"/>
      <div v-if="!isLoading" class="vertical item" id="diff-commit-list-show-all" role="menuitem" @keydown.enter="showAllChanges()" @click="showAllChanges()">
        <div class="gt-ellipsis">
          {{ locale.show_all_commits }}
        </div>
        <div class="gt-ellipsis text light-2 tw-mb-0">
          {{ locale.stats_num_commits }}
        </div>
      </div>
      <!-- only show the show changes since last review if there is a review AND we are commits ahead of the last review -->
      <div
        v-if="lastReviewCommitSha != null" role="menuitem"
        class="vertical item"
        :class="{disabled: !commitsSinceLastReview}"
        @keydown.enter="changesSinceLastReviewClick()"
        @click="changesSinceLastReviewClick()"
      >
        <div class="gt-ellipsis">
          {{ locale.show_changes_since_your_last_review }}
        </div>
        <div class="gt-ellipsis text light-2">
          {{ commitsSinceLastReview }} commits
        </div>
      </div>
      <span v-if="!isLoading" class="info text light-2">{{ locale.select_commit_hold_shift_for_range }}</span>
      <template v-for="commit in commits" :key="commit.id">
        <div
          class="vertical item" role="menuitem"
          :class="{selection: commit.selected, hovered: commit.hovered}"
          @keydown.enter.exact="commitClicked(commit.id)"
          @keydown.enter.shift.exact="commitClickedShift(commit)"
          @mouseover.shift="highlight(commit)"
          @click.exact="commitClicked(commit.id)"
          @click.ctrl.exact="commitClicked(commit.id, true)"
          @click.meta.exact="commitClicked(commit.id, true)"
          @click.shift.exact.stop.prevent="commitClickedShift(commit)"
        >
          <div class="tw-flex-1 tw-flex tw-flex-col tw-gap-1">
            <div class="gt-ellipsis commit-list-summary">
              {{ commit.summary }}
            </div>
            <div class="gt-ellipsis text light-2">
              {{ commit.committer_or_author_name }}
              <span class="text right">
                <!-- TODO: make this respect the PreferredTimestampTense setting -->
                <relative-time prefix="" :datetime="commit.time" data-tooltip-content data-tooltip-interactive="true">{{ commit.time }}</relative-time>
              </span>
            </div>
          </div>
          <div class="tw-font-mono">
            {{ commit.short_sha }}
          </div>
        </div>
      </template>
    </div>
  </div>
</template>
<style scoped>
  .hovered:not(.selection) {
    background-color: var(--color-small-accent) !important;
  }
  .selection {
    background-color: var(--color-accent) !important;
  }

  .info {
    display: inline-block;
    padding: 7px 14px !important;
    line-height: 1.4;
    width: 100%;
  }

  #diff-commit-selector-menu {
    overflow-x: hidden;
    max-height: 450px;
  }

  #diff-commit-selector-menu .loading-indicator {
    height: 200px;
    width: 350px;
  }

  #diff-commit-selector-menu .item,
  #diff-commit-selector-menu .info {
    display: flex !important;
    flex-direction: row;
    line-height: 1.4;
    padding: 7px 14px !important;
    border-top: 1px solid var(--color-secondary) !important;
    gap: 0.25em;
  }

  #diff-commit-selector-menu .item:focus {
    color: var(--color-text);
    background: var(--color-hover);
  }

  #diff-commit-selector-menu .commit-list-summary {
    max-width: min(380px, 96vw);
  }
</style>
