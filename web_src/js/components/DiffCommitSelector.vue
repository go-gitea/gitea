<script lang="ts" setup>
import {computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, useTemplateRef, type ShallowRef} from 'vue';
import SvgIcon from './SvgIcon.vue';
import {GET} from '../modules/fetch.ts';
import {generateElemId} from '../utils/dom.ts';

type Commit = {
  id: string,
  hovered: boolean,
  selected: boolean,
  summary: string,
  committer_or_author_name: string,
  time: string,
  short_sha: string,
};

type CommitListResult = {
  commits: Array<Commit>,
  last_review_commit_sha: string,
  locale: Record<string, string>,
};

const elRoot = useTemplateRef('elRoot') as Readonly<ShallowRef<HTMLDivElement>>;
const elExpandBtn = useTemplateRef('elExpandBtn') as Readonly<ShallowRef<HTMLButtonElement>>;
const elShowAllChanges = useTemplateRef('elShowAllChanges') as Readonly<ShallowRef<HTMLDivElement>>;
const elMenu = useTemplateRef('elMenu') as Readonly<ShallowRef<HTMLDivElement>>;

const elMount = document.querySelector('#diff-commit-select')!;
const queryParams = elMount.getAttribute('data-queryparams');
const issueLink = elMount.getAttribute('data-issuelink');
const mergeBase = elMount.getAttribute('data-merge-base');
const beforeCommit = elMount.getAttribute('data-before-commit');
const afterCommit = elMount.getAttribute('data-after-commit');
const uniqueIdMenu = generateElemId('diff-commit-selector-menu-');

const menuVisible = shallowRef(false);
const isLoading = shallowRef(false);
const locale = shallowRef<Record<string, string>>({filter_changes_by_commit: elMount.getAttribute('data-text-filter-changes-by-commit')!});
const commits = ref<Array<Commit>>([]); // deep, the commit objects are mutated in place
const hoverActivated = shallowRef(false);
const lastReviewCommitSha = shallowRef<string | null>(null);
const activeRange = shallowRef<[number, number] | null>(null);

function isActive(idx: number) {
  return activeRange.value !== null && activeRange.value[0] <= idx && idx <= activeRange.value[1];
}

const commitsSinceLastReview = computed(() => {
  if (lastReviewCommitSha.value) {
    return commits.value.length - commits.value.findIndex((x) => x.id === lastReviewCommitSha.value) - 1;
  }
  return 0;
});

onMounted(() => {
  document.body.addEventListener('click', onBodyClick);
  elRoot.value.addEventListener('keydown', onKeyDown);
  elRoot.value.addEventListener('keyup', onKeyUp);
});

onBeforeUnmount(() => { // template refs are null by onUnmounted
  document.body.removeEventListener('click', onBodyClick);
  elRoot.value.removeEventListener('keydown', onKeyDown);
  elRoot.value.removeEventListener('keyup', onKeyUp);
});

function onBodyClick(event: MouseEvent) {
  // close this menu on click outside of this element when the dropdown is currently visible opened
  if (elRoot.value.contains(event.target as Node)) return;
  if (menuVisible.value) {
    toggleMenu();
  }
}

function onKeyDown(event: KeyboardEvent) {
  if (!menuVisible.value || event.isComposing) return;
  const item = document.activeElement as HTMLElement;
  if (!elRoot.value.contains(item)) return;
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault();
    const items = Array.from(elRoot.value.querySelectorAll<HTMLElement>('.dropdown-item:not([aria-disabled="true"])'));
    items[items.indexOf(item) + (event.key === 'ArrowDown' ? 1 : -1)]?.focus();
    const commitIdx = document.activeElement!.getAttribute('data-commit-idx');
    if (commitIdx) highlight(commits.value[Number(commitIdx)]);
  } else if (event.key === 'Escape') {
    event.preventDefault();
    toggleMenu();
  }
}

function onKeyUp(event: KeyboardEvent) {
  if (!menuVisible.value) return;
  const item = document.activeElement;
  if (!elRoot.value.contains(item)) return;
  if (event.key === 'Shift' && hoverActivated.value) {
    // shift is not pressed anymore -> deactivate hovering and reset hovered and selected
    hoverActivated.value = false;
    for (const commit of commits.value) {
      commit.hovered = false;
      commit.selected = false;
    }
  }
}

function highlight(commit: Commit) {
  if (!hoverActivated.value) return;
  const indexSelected = commits.value.findIndex((x) => x.selected);
  const indexCurrentElem = commits.value.findIndex((x) => x.id === commit.id);
  for (const [idx, commit] of commits.value.entries()) {
    commit.hovered = Math.min(indexSelected, indexCurrentElem) <= idx && idx <= Math.max(indexSelected, indexCurrentElem);
  }
}

async function toggleMenu() {
  menuVisible.value = !menuVisible.value;
  if (!commits.value.length && menuVisible.value && !isLoading.value) {
    isLoading.value = true;
    try {
      await fetchCommits();
    } finally {
      isLoading.value = false;
    }
  }
  nextTick(() => {
    if (menuVisible.value) {
      elShowAllChanges.value?.focus();
      const elActive = elMenu.value.querySelector<HTMLElement>('.dropdown-item.active');
      if (elActive) elMenu.value.scrollTop = elActive.offsetTop - elMenu.value.clientHeight / 2; // scrollIntoView would also scroll the page
    } else {
      elExpandBtn.value.focus();
    }
  });
}

/** Load the commits to show in this dropdown */
async function fetchCommits() {
  const resp = await GET(`${issueLink}/commits/list`);
  const results = await resp.json() as CommitListResult;
  for (const commit of results.commits) commit.hovered = false;
  commits.value.push(...results.commits);
  commits.value.reverse();
  lastReviewCommitSha.value = results.last_review_commit_sha || null;
  if (lastReviewCommitSha.value && commits.value.every((x) => x.id !== lastReviewCommitSha.value)) {
    // the lastReviewCommit is not available (probably due to a force push)
    // reset the last review commit sha
    lastReviewCommitSha.value = null;
  }
  locale.value = {...locale.value, ...results.locale};
  const end = commits.value.findIndex((x) => x.id === afterCommit);
  if (end >= 0) activeRange.value = [commits.value.findIndex((x) => x.id === beforeCommit) + 1, end]; // starts at 0 when beforeCommit is outside the list
}

function showAllChanges() {
  window.location.assign(`${issueLink}/files${queryParams}`);
}

/** Called when user clicks on since last review */
function changesSinceLastReviewClick() {
  window.location.assign(`${issueLink}/files/${lastReviewCommitSha.value}..${commits.value.at(-1)!.id}${queryParams}`);
}

/** Clicking on a single commit opens this specific commit */
function commitClicked(commitId: string, newWindow = false) {
  const url = `${issueLink}/commits/${commitId}${queryParams}`;
  if (newWindow) {
    window.open(url);
  } else {
    window.location.assign(url);
  }
}

/**
 * When a commit is clicked while holding Shift, it enables range selection.
 * - The range selection is a half-open, half-closed range, meaning it excludes the start commit but includes the end commit.
 * - The start of the commit range is always the previous commit of the first clicked commit.
 * - If the first commit in the list is clicked, the mergeBase will be used as the start of the range instead.
 * - The second Shift-click defines the end of the range.
 * - Once both are selected, the diff view for the selected commit range will open.
 */
function commitClickedShift(commit: Commit) {
  hoverActivated.value = !hoverActivated.value;
  commit.selected = true;
  // Second click -> determine our range and open links accordingly
  if (!hoverActivated.value) {
    // since at least one commit is selected, we can determine the range
    // find all selected commits and generate a link
    const firstSelected = commits.value.findIndex((x) => x.selected);
    const lastSelected = commits.value.findLastIndex((x) => x.selected);
    const beforeCommitID = firstSelected === 0 ? mergeBase : commits.value[firstSelected - 1].id;
    const afterCommitID = commits.value[lastSelected].id;

    if (firstSelected === lastSelected) {
      // if the start and end are the same, we show this single commit
      window.location.assign(`${issueLink}/commits/${afterCommitID}${queryParams}`);
    } else if (beforeCommitID === mergeBase && afterCommitID === commits.value.at(-1)!.id) {
      // if the first commit is selected and the last commit is selected, we show all commits
      window.location.assign(`${issueLink}/files${queryParams}`);
    } else {
      window.location.assign(`${issueLink}/files/${beforeCommitID}..${afterCommitID}${queryParams}`);
    }
  }
}
</script>
<template>
  <div class="diff-commit-selector tw-relative tw-inline-flex" ref="elRoot">
    <button
      ref="elExpandBtn"
      type="button"
      class="ui tiny basic button"
      @click.stop="toggleMenu()"
      :data-tooltip-content="locale.filter_changes_by_commit"
      aria-haspopup="menu"
      :aria-expanded="menuVisible"
      :aria-label="locale.filter_changes_by_commit"
      :aria-controls="uniqueIdMenu"
    >
      <svg-icon name="octicon-git-commit"/>
    </button>
    <div class="dropdown-popup dropdown-popup-flex" ref="elMenu" :id="uniqueIdMenu" role="menu" v-show="menuVisible" v-cloak>
      <div class="loading-indicator is-loading" v-if="isLoading"/>
      <div v-if="!isLoading" class="dropdown-item" ref="elShowAllChanges" role="menuitem" tabindex="-1" @keydown.enter="showAllChanges()" @click="showAllChanges()">
        <div class="gt-ellipsis">
          {{ locale.show_all_commits }}
        </div>
        <div class="gt-ellipsis tw-text-text-light-2 tw-mb-0">
          {{ locale.stats_num_commits }}
        </div>
      </div>
      <!-- only show the show changes since last review if there is a review AND we are commits ahead of the last review -->
      <div
        v-if="lastReviewCommitSha != null"
        class="dropdown-item" role="menuitem" tabindex="-1"
        :aria-disabled="!commitsSinceLastReview"
        @keydown.enter="commitsSinceLastReview && changesSinceLastReviewClick()"
        @click="commitsSinceLastReview && changesSinceLastReviewClick()"
      >
        <div class="gt-ellipsis">
          {{ locale.show_changes_since_your_last_review }}
        </div>
        <div class="gt-ellipsis tw-text-text-light-2">
          {{ commitsSinceLastReview }} commits
        </div>
      </div>
      <div v-if="!isLoading" class="dropdown-message">{{ locale.select_commit_hold_shift_for_range }}</div>
      <template v-for="(commit, idx) in commits" :key="commit.id">
        <div
          class="dropdown-item" role="menuitem" tabindex="-1"
          :class="{selected: commit.selected, hovered: commit.hovered, active: isActive(idx)}"
          :aria-current="isActive(idx) || undefined"
          :data-commit-idx="idx"
          @keydown.enter.exact="commitClicked(commit.id)"
          @keydown.enter.shift.exact="commitClickedShift(commit)"
          @mouseover.shift="highlight(commit)"
          @click.exact="commitClicked(commit.id)"
          @click.ctrl.exact="commitClicked(commit.id, true)"
          @click.meta.exact="commitClicked(commit.id, true)"
          @click.shift.exact.stop.prevent="commitClickedShift(commit)"
        >
          <div class="tw-flex-1 tw-min-w-0 tw-flex tw-flex-col tw-gap-1">
            <div class="gt-ellipsis commit-list-summary">
              {{ commit.summary }}
            </div>
            <div class="gt-ellipsis tw-text-text-light-2">
              {{ commit.committer_or_author_name }}
              <span class="text right">
                <!-- TODO: make this respect the PreferredTimestampTense setting -->
                <relative-time prefix="" :datetime="commit.time" data-tooltip-content data-tooltip-interactive="true">{{ commit.time }}</relative-time>
              </span>
            </div>
          </div>
          <div class="tw-font-mono tw-shrink-0 flex-text-block">
            <svg-icon name="octicon-check" :size="14" v-if="isActive(idx)"/>
            {{ commit.short_sha }}
          </div>
        </div>
      </template>
    </div>
  </div>
</template>
<style scoped>
  .dropdown-popup {
    position: absolute;
    z-index: 11;
    top: 100%;
    right: 0;
    width: max-content;
    max-width: calc(100vw - 16px);
    overflow-x: hidden;
  }

  .loading-indicator {
    height: 200px;
    width: 350px;
  }

  .dropdown-item:focus {
    background: var(--color-active);
  }

  .dropdown-item.hovered {
    background-color: var(--color-small-accent);
  }

  .dropdown-item.selected {
    background-color: var(--color-accent);
  }

  .commit-list-summary {
    max-width: min(380px, 96vw);
  }
</style>
