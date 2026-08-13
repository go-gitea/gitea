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
}

type CommitListResult = {
  commits: Array<Commit>,
  last_review_commit_sha: string,
  locale: Record<string, string>,
}

const elRoot = useTemplateRef('elRoot') as Readonly<ShallowRef<HTMLDivElement>>;
const elExpandBtn = useTemplateRef('elExpandBtn') as Readonly<ShallowRef<HTMLButtonElement>>;
const elShowAllChanges = useTemplateRef('elShowAllChanges') as Readonly<ShallowRef<HTMLDivElement>>;

const elMount = document.querySelector('#diff-commit-select')!;
const queryParams = elMount.getAttribute('data-queryparams');
const issueLink = elMount.getAttribute('data-issuelink');
const mergeBase = elMount.getAttribute('data-merge-base');
const uniqueIdMenu = generateElemId('diff-commit-selector-menu-');
const uniqueIdShowAll = generateElemId('diff-commit-selector-show-all-');

const menuVisible = shallowRef(false);
const isLoading = shallowRef(false);
const locale = shallowRef<Record<string, string>>({filter_changes_by_commit: elMount.getAttribute('data-filter_changes_by_commit')!});
const commits = ref<Array<Commit>>([]); // deep, the commit objects are mutated in place
const hoverActivated = shallowRef(false);
const lastReviewCommitSha = shallowRef<string | null>(null);

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
  if (!menuVisible.value) return;
  const item = document.activeElement as HTMLElement;
  if (!elRoot.value.contains(item)) return;
  switch (event.key) {
    case 'ArrowDown': // select next element
      event.preventDefault();
      focusElem(item.nextElementSibling as HTMLElement, item);
      break;
    case 'ArrowUp': // select previous element
      event.preventDefault();
      focusElem(item.previousElementSibling as HTMLElement, item);
      break;
    case 'Escape': // close menu
      event.preventDefault();
      item.tabIndex = -1;
      toggleMenu();
      break;
  }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    const item = document.activeElement; // try to highlight the selected commits
    const commitIdx = item?.matches('.item') ? item.getAttribute('data-commit-idx') : null;
    if (commitIdx) highlight(commits.value[Number(commitIdx)]);
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

/** Focus given element */
function focusElem(elem: HTMLElement, prevElem: HTMLElement) {
  if (elem) {
    elem.tabIndex = 0;
    if (prevElem) prevElem.tabIndex = -1;
    elem.focus();
  }
}

/** Opens our menu, loads commits before opening */
async function toggleMenu() {
  menuVisible.value = !menuVisible.value;
  // load our commits when the menu is not yet visible (it'll be toggled after loading)
  // and we got no commits
  if (!commits.value.length && menuVisible.value && !isLoading.value) {
    isLoading.value = true;
    try {
      await fetchCommits();
    } finally {
      isLoading.value = false;
    }
  }
  // set correct tabindex to allow easier navigation
  nextTick(() => {
    if (menuVisible.value) {
      focusElem(elShowAllChanges.value, elExpandBtn.value);
    } else {
      focusElem(elExpandBtn.value, elShowAllChanges.value);
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
  if (lastReviewCommitSha.value && !commits.value.some((x) => x.id === lastReviewCommitSha.value)) {
    // the lastReviewCommit is not available (probably due to a force push)
    // reset the last review commit sha
    lastReviewCommitSha.value = null;
  }
  locale.value = {...locale.value, ...results.locale};
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
  <div class="ui scrolling dropdown custom diff-commit-selector" ref="elRoot">
    <button
      ref="elExpandBtn"
      class="ui tiny basic button"
      @click.stop="toggleMenu()"
      :data-tooltip-content="locale.filter_changes_by_commit"
      aria-haspopup="true"
      :aria-label="locale.filter_changes_by_commit"
      :aria-controls="uniqueIdMenu"
      :aria-activedescendant="uniqueIdShowAll"
    >
      <svg-icon name="octicon-git-commit"/>
    </button>
    <!-- this dropdown is not managed by Fomantic UI, so it needs some classes like "transition" explicitly -->
    <div class="left menu transition" :id="uniqueIdMenu" :class="{visible: menuVisible}" v-show="menuVisible" v-cloak :aria-expanded="menuVisible ? 'true': 'false'">
      <div class="loading-indicator is-loading" v-if="isLoading"/>
      <div v-if="!isLoading" class="item" :id="uniqueIdShowAll" ref="elShowAllChanges" role="menuitem" @keydown.enter="showAllChanges()" @click="showAllChanges()">
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
        class="item" role="menuitem"
        :class="{disabled: !commitsSinceLastReview}"
        @keydown.enter="changesSinceLastReviewClick()"
        @click="changesSinceLastReviewClick()"
      >
        <div class="gt-ellipsis">
          {{ locale.show_changes_since_your_last_review }}
        </div>
        <div class="gt-ellipsis tw-text-text-light-2">
          {{ commitsSinceLastReview }} commits
        </div>
      </div>
      <span v-if="!isLoading" class="info tw-text-text-light-2">{{ locale.select_commit_hold_shift_for_range }}</span>
      <template v-for="(commit, idx) in commits" :key="commit.id">
        <div
          class="item" role="menuitem"
          :class="{selected: commit.selected, hovered: commit.hovered}"
          :data-commit-idx="idx"
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
            <div class="gt-ellipsis tw-text-text-light-2">
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
  .ui.dropdown.diff-commit-selector .menu {
    margin-top: 0.25em;
    overflow-x: hidden;
    max-height: 450px;
  }

  .ui.dropdown.diff-commit-selector .menu .loading-indicator {
    height: 200px;
    width: 350px;
  }

  .ui.dropdown.diff-commit-selector .menu > .item,
  .ui.dropdown.diff-commit-selector .menu > .info {
    display: flex;
    flex-direction: row;
    line-height: 1.4;
    gap: 0.25em;
    padding: 7px 14px !important;
  }

  .ui.dropdown.diff-commit-selector .menu > .item:not(:first-child),
  .ui.dropdown.diff-commit-selector .menu > .info:not(:first-child) {
    border-top: 1px solid var(--color-secondary) !important;
  }

  .ui.dropdown.diff-commit-selector .menu > .item:focus {
    background: var(--color-active);
  }

  .ui.dropdown.diff-commit-selector .menu > .item.hovered {
    background-color: var(--color-small-accent);
  }

  .ui.dropdown.diff-commit-selector .menu > .item.selected {
    background-color: var(--color-accent);
  }

  .ui.dropdown.diff-commit-selector .menu .commit-list-summary {
    max-width: min(380px, 96vw);
  }
</style>
