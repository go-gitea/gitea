<template>
  <div class="ui dropdown custom">
    <button
      class="ui basic button"
      id="expand-button"
      @click.stop="toggleMenu()"
      :data-tooltip-content="locale.filter_changes_by_commit"
      aria-haspopup="true"
      tabindex="0"
      aria-controls="commit-selector-menu"
      :aria-label="locale.filter_changes_by_commit"
      aria-activedescendant="show-all-changes"
    >
      <svg-icon name="octicon-git-commit"/>
    </button>
    <div class="menu left transition commit-selector-menu" id="commit-selector-menu" :class="{visible: menuVisible}" v-show="menuVisible" v-cloak :aria-expanded="menuVisible ? 'true': 'false'">
      <div class="scrolling menu gt-border-t-0" :class="{'is-loading': isLoading}">
        <div class="vertical item gt-df gt-fc gt-gap-2" id="show-all-changes" @keydown.enter="showAllChanges()" @click="showAllChanges()" role="menuitem" tabindex="-1">
          <div class="gt-ellipsis">
            {{ locale.show_all_commits }}
          </div>
          <div class="gt-ellipsis text light-2 gt-mb-0">
            {{ locale.stats_num_commits }}
          </div>
        </div>
        <!-- only show the show changes since last review if there is a review AND we are commits ahead of the last review -->
        <div v-if="lastReviewCommitSha != null" role="menuitem" tabindex="-1" class="vertical item gt-df gt-fc gt-gap-2 gt-border-secondary-top" :class="{disabled: commitsSinceLastReview === 0}" @keydown.enter="changesSinceLastReviewClick()" @click="changesSinceLastReviewClick()">
          <div class="gt-ellipsis">
            {{ locale.show_changes_since_your_last_review }}
          </div>
          <div class="gt-ellipsis text light-2">
            {{ commitsSinceLastReview }} commits
          </div>
        </div>
        <span class="info gt-border-secondary-top text light-2">{{ locale.select_commit_hold_shift_for_range }}</span>
        <template v-for="commit in commits" :key="commit.id">
          <div class="vertical item gt-df gt-fr gt-gap-2 gt-border-secondary-top" role="menuitem" tabindex="-1" :class="{selected: commit.selected}" @keydown.enter.exact="commitClicked(commit.id)" @click.exact="commitClicked(commit.id)" @keydown.enter.shift.exact="commitClickedShift(commit)" @click.shift.exact.stop.prevent="commitClickedShift(commit)">
            <div class="gt-f1 gt-df gt-fc gt-gap-2">
              <div class="gt-ellipsis commit-list-summary">
                {{ commit.summary }}
              </div>
              <div class="gt-ellipsis text light-2">
                {{ commit.committer_or_author_name }}
                <span class="text right">
                  <relative-time class="time-since" prefix="" :datetime="commit.time" data-tooltip-content data-tooltip-interactive="true">{{ commit.time }}</relative-time>
                </span>
              </div>
            </div>
            <div class="gt-mono">
              {{ commit.short_sha }}
            </div>
          </div>
        </template>
      </div>
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
      isLoading: false,
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
  mounted() {
    // Add a click listener to close this menu on click outside of this element
    //  when the dropdown is currently visible opened
    document.body.addEventListener('click', (event) => {
      if (this.$el.contains(event.target)) return;
      if (this.menuVisible) {
        this.toggleMenu();
      }
    });

    this.$el.addEventListener('keyup', (event) => {
      if (!this.menuVisible) return;
      const item = document.activeElement;
      switch (event.key) {
        case 'ArrowDown':
          // Arrowdown -> select next element
          event.preventDefault();
          this.focusElem(item.nextElementSibling, item);
          break;
        case 'ArrowUp':
          // ArrowUp -> select previous element
          event.preventDefault();
          this.focusElem(item.previousElementSibling, item);
          break;
        case 'Escape':
          // Escape -> close menu
          item.tabIndex = -1;
          this.toggleMenu();
          break;
      }
    });
  },
  methods: {
    /** Focus given element */
    focusElem(elem, prevElem) {
      if (elem) {
        elem.tabIndex = 0;
        prevElem.tabIndex = -1;
        elem.focus();
      }
    },
    /** Opens our menu, loads commits before opening */
    async toggleMenu() {
      this.menuVisible = !this.menuVisible;
      // load our commits when the menu is not yet visible (it'll be toggled after loading)
      // and we got no commits
      if (this.commits.length === 0 && this.menuVisible === true) {
        this.isLoading = true;
        await this.fetchCommits();
        this.isLoading = false;
      }
      // set correct tabindex to allow easier navigation
      this.$nextTick(() => {
        const expandBtn = this.$el.querySelector('#expand-button');
        const showAllChanges = this.$el.querySelector('#show-all-changes');
        if (this.menuVisible) {
          this.focusElem(showAllChanges, expandBtn);
        } else {
          this.focusElem(expandBtn, showAllChanges);
        }
      });
    },
    /** Load the commits to show in this dropdown */
    async fetchCommits() {
      const resp = await fetch(`${this.issueLink}/commits/list`);
      const results = await resp.json();
      this.commits.push(...results.commits);
      this.commits.reverse();
      this.lastReviewCommitSha = results.last_review_commit_sha || null;
      if (this.lastReviewCommitSha && this.commits.findIndex((x) => x.id === this.lastReviewCommitSha) === -1) {
        // the lastreviewcommit is not available (probably due to a force push)
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

  .ui.dropdown .menu .menu.is-loading {
    left: 0;
    height: 200px;
    width: 200px !important;
  }

  .ui.dropdown .menu .menu.is-loading::after {
    display: block !important; /* to override fomantic rule .ui.dropdown .menu .menu:after {display: none} */
  }

  .commit-selector-menu {
    overflow-x: hidden;
    border-top: 0;
  }

  .commit-selector-menu .scrolling.menu {
    max-height: 450px !important;
  }

  .ui.dropdown .menu.commit-selector-menu > .item {
    line-height: 1.4;
    padding: 7px 14px !important;
  }

  .commit-list-summary {
    max-width: min(380px, 96vw);
  }

  .item:focus {
    color: var(--color-text);
    background: var(--color-hover);
  }
</style>
