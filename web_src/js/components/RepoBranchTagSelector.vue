<script lang="ts" setup>
import {computed, nextTick, onBeforeUnmount, onMounted, shallowRef, useTemplateRef, watch, type ShallowRef} from 'vue';
import SvgIcon from './SvgIcon.vue';
import {showErrorToast} from '../modules/toast.ts';
import {errorMessage} from '../modules/errors.ts';
import {GET} from '../modules/fetch.ts';
import {pathEscapeSegments} from '../utils/url.ts';
import {generateElemId, queryElemChildren} from '../utils/dom.ts';
import type {GitRefType} from '../types.ts';
import {trString} from '../modules/i18n.ts';

type ListItem = {
  selected: boolean;
  refShortName: string;
  refType: GitRefType;
  rssFeedLink: string;
};

type SelectedTab = 'branches' | 'tags';

type TabLoadingStates = Record<SelectedTab, '' | 'loading' | 'done'>;

const props = defineProps<{
  elRoot: HTMLElement;
}>();

const elDropdown = useTemplateRef('elDropdown') as Readonly<ShallowRef<HTMLDivElement>>;
const elCreateNewRefForm = useTemplateRef('elCreateNewRefForm') as Readonly<ShallowRef<HTMLFormElement>>;
const elScrollContainer = useTemplateRef('elScrollContainer') as Readonly<ShallowRef<HTMLDivElement>>;
const elSearchField = useTemplateRef('elSearchField') as Readonly<ShallowRef<HTMLInputElement>>;
const elTrigger = useTemplateRef<HTMLButtonElement>('elTrigger');
const popupId = generateElemId('branch-tag-menu-');

const showTabBranches = props.elRoot.getAttribute('data-show-tab-branches') === 'true';

const allItems = shallowRef<ListItem[]>([]);
const selectedTab = shallowRef<SelectedTab>(showTabBranches ? 'branches' : 'tags');
const searchTerm = shallowRef('');
const menuVisible = shallowRef(false);
const activeItemIndex = shallowRef(-1);
const tabLoadingStates = shallowRef<TabLoadingStates>({branches: '', tags: ''});

const textBranches = props.elRoot.getAttribute('data-text-branches')!;
const textTags = props.elRoot.getAttribute('data-text-tags')!;
const textFilterBranch = props.elRoot.getAttribute('data-text-filter-branch')!;
const textFilterTag = props.elRoot.getAttribute('data-text-filter-tag')!;
const textDefaultBranchLabel = props.elRoot.getAttribute('data-text-default-branch-label')!;
const textCreateTag = props.elRoot.getAttribute('data-text-create-tag')!;
const textCreateBranch = props.elRoot.getAttribute('data-text-create-branch')!;
const textCreateRefFrom = props.elRoot.getAttribute('data-text-create-ref-from')!;
const textNoResults = props.elRoot.getAttribute('data-text-no-results')!;
const textViewAllBranches = props.elRoot.getAttribute('data-text-view-all-branches')!;
const textViewAllTags = props.elRoot.getAttribute('data-text-view-all-tags')!;

const currentRepoDefaultBranch = props.elRoot.getAttribute('data-current-repo-default-branch')!;
const currentRepoLink = props.elRoot.getAttribute('data-current-repo-link')!;
const currentTreePath = props.elRoot.getAttribute('data-current-tree-path')!;
const currentRefType = shallowRef(props.elRoot.getAttribute('data-current-ref-type') as GitRefType);
const currentRefShortName = shallowRef(props.elRoot.getAttribute('data-current-ref-short-name')!);

const refLinkTemplate = props.elRoot.getAttribute('data-ref-link-template')!;
const refFormActionTemplate = props.elRoot.getAttribute('data-ref-form-action-template')!;
const dropdownFixedText = props.elRoot.getAttribute('data-dropdown-fixed-text')!;
const showTabTags = props.elRoot.getAttribute('data-show-tab-tags') === 'true';
const allowCreateNewRef = props.elRoot.getAttribute('data-allow-create-new-ref') === 'true';
const showViewAllRefsEntry = props.elRoot.getAttribute('data-show-view-all-refs-entry') === 'true';
const enableFeed = props.elRoot.getAttribute('data-enable-feed') === 'true';

const searchFieldPlaceholder = computed(() => selectedTab.value === 'branches' ? textFilterBranch : textFilterTag);

const showCreateNewRef = computed(() => {
  if (!allowCreateNewRef || !searchTerm.value) {
    return false;
  }
  // FIXME: not quite right here, it mixes "branch" and "tag" names
  return allItems.value.every((item: ListItem) => item.refShortName !== searchTerm.value);
});

const filteredItems = computed<ListItem[]>(() => {
  const searchTermLower = searchTerm.value.toLowerCase();
  return allItems.value.filter((item: ListItem) => {
    const typeMatched = (selectedTab.value === 'branches' && item.refType === 'branch') || (selectedTab.value === 'tags' && item.refType === 'tag');
    if (!typeMatched) return false;
    if (!searchTerm.value) return true; // match all
    return item.refShortName.toLowerCase().includes(searchTermLower);
  });
});

watch(filteredItems, (items) => {
  activeItemIndex.value = !items.length && showCreateNewRef.value ? 0 : -1;
}, {flush: 'sync'});

const showNoResults = computed(() => {
  if (tabLoadingStates.value[selectedTab.value] !== 'done') return false;
  return !filteredItems.value.length && !showCreateNewRef.value;
});

const createNewRefFormActionUrl = computed(() => {
  return `${currentRepoLink}/branches/_new/${currentRefType.value}/${pathEscapeSegments(currentRefShortName.value)}`;
});

watch(menuVisible, async (visible: boolean) => {
  if (!visible) return;
  focusSearchField();
  await loadTabItems();
  await nextTick();
  if (menuVisible.value) elScrollContainer.value.closest('.dropdown-popup')!.scrollIntoView({block: 'nearest'});
});

function onBodyClick(e: MouseEvent) {
  if (elDropdown.value.contains(e.target as Node)) return;
  if (menuVisible.value) menuVisible.value = false;
}

onMounted(() => {
  document.body.addEventListener('click', onBodyClick);
  if (refFormActionTemplate) {
    // if the selector is used in a form and needs to change the form action,
    // make a mock item and select it to update the form action
    const item: ListItem = {selected: true, refType: currentRefType.value, refShortName: currentRefShortName.value, rssFeedLink: ''};
    selectItem(item);
  }
});

onBeforeUnmount(() => { // template refs are null by onUnmounted
  document.body.removeEventListener('click', onBodyClick);
});

function selectItem(item: ListItem) {
  menuVisible.value = false;
  if (refFormActionTemplate) {
    currentRefType.value = item.refType;
    currentRefShortName.value = item.refShortName;
    elDropdown.value.closest('form')!.action = refFormActionTemplate
      .replace('{RepoLink}', () => currentRepoLink)
      .replace('{RefType}', () => pathEscapeSegments(item.refType))
      .replace('{RefShortName}', () => pathEscapeSegments(item.refShortName));
  } else {
    window.location.assign(refLinkTemplate
      .replace('{RepoLink}', () => currentRepoLink)
      .replace('{RefType}', () => pathEscapeSegments(item.refType))
      .replace('{RefShortName}', () => pathEscapeSegments(item.refShortName))
      .replace('{TreePath}', () => pathEscapeSegments(currentTreePath)));
  }
}

function createNewRef() {
  elCreateNewRefForm.value?.submit();
}

function focusSearchField() {
  nextTick(() => {
    elSearchField.value.focus();
  });
}

function getSelectedIndexInFiltered() {
  return filteredItems.value.findIndex((item) => item.selected);
}

function getActiveItem() {
  return queryElemChildren<HTMLDivElement>(elScrollContainer.value, '.dropdown-item')[activeItemIndex.value] ?? null;
}

function keydown(e: KeyboardEvent) {
  if (e.isComposing) return;
  if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
    e.preventDefault();

    if (activeItemIndex.value === -1) {
      activeItemIndex.value = getSelectedIndexInFiltered();
    }
    const nextIndex = e.key === 'ArrowDown' ? activeItemIndex.value + 1 : activeItemIndex.value - 1;
    if (nextIndex < 0) {
      return;
    }
    if (nextIndex + (showCreateNewRef.value ? 0 : 1) > filteredItems.value.length) {
      return;
    }
    activeItemIndex.value = nextIndex;
    getActiveItem().scrollIntoView({block: 'nearest'});
  } else if (e.key === 'Enter') {
    e.preventDefault();
    getActiveItem()?.click();
  } else if (e.key === 'Escape') {
    e.preventDefault();
    menuVisible.value = false;
    elTrigger.value!.focus();
  }
}

function handleTabSwitch(tab: SelectedTab) {
  selectedTab.value = tab;
  focusSearchField();
  loadTabItems();
}

async function loadTabItems() {
  const tab = selectedTab.value;
  if (tabLoadingStates.value[tab] === 'loading' || tabLoadingStates.value[tab] === 'done') return;

  const refType = tab === 'branches' ? 'branch' : 'tag';
  tabLoadingStates.value = {...tabLoadingStates.value, [tab]: 'loading'};
  try {
    const resp = await GET(`${currentRepoLink}/${tab}/list`);
    const {results} = await resp.json() as {results: string[]};
    allItems.value = [...allItems.value, ...results.map((refShortName): ListItem => ({
      refType,
      refShortName,
      selected: refType === currentRefType.value && refShortName === currentRefShortName.value,
      rssFeedLink: `${currentRepoLink}/rss/${refType}/${pathEscapeSegments(refShortName)}`,
    }))];
    tabLoadingStates.value = {...tabLoadingStates.value, [tab]: 'done'};
  } catch (e) {
    tabLoadingStates.value = {...tabLoadingStates.value, [tab]: ''};
    showErrorToast(`Network error when fetching items for ${tab}, error: ${errorMessage(e)}`);
    console.error(e);
  }
}
</script>
<template>
  <div class="branch-selector-dropdown" ref="elDropdown">
    <button
      type="button" class="ui compact button branch-dropdown-button" ref="elTrigger"
      aria-haspopup="menu" :aria-expanded="menuVisible" :aria-controls="popupId"
      @click="menuVisible = !menuVisible" @keydown.down.prevent="menuVisible = true" @keydown.up.prevent="menuVisible = true"
    >
      <span class="flex-text-block gt-ellipsis">
        <template v-if="dropdownFixedText">{{ dropdownFixedText }}</template>
        <template v-else>
          <svg-icon v-if="currentRefType === 'tag'" name="octicon-tag"/>
          <svg-icon v-else-if="currentRefType === 'branch'" name="octicon-git-branch"/>
          <svg-icon v-else name="octicon-git-commit"/>
          <strong class="tw-inline-block gt-ellipsis">{{ currentRefShortName }}</strong>
        </template>
      </span>
      <svg-icon name="octicon-triangle-down" :size="14"/>
    </button>
    <div class="dropdown-popup" :id="popupId" role="menu" v-show="menuVisible" v-cloak>
      <div class="ui icon search input">
        <i class="icon"><svg-icon name="octicon-filter" :size="16"/></i>
        <input type="text" class="dropdown-filter" ref="elSearchField" autocomplete="off" v-model="searchTerm" @keydown="keydown($event)" :placeholder="searchFieldPlaceholder" :aria-label="searchFieldPlaceholder">
      </div>
      <div v-if="showTabBranches" class="branch-tag-tab flex-text-block tw-gap-0">
        <button type="button" class="btn branch-tag-item" :class="{active: selectedTab === 'branches'}" @click="handleTabSwitch('branches')">
          <svg-icon name="octicon-git-branch" :size="16" class="tw-mr-1"/>{{ textBranches }}
        </button>
        <button v-if="showTabTags" type="button" class="btn branch-tag-item" :class="{active: selectedTab === 'tags'}" @click="handleTabSwitch('tags')">
          <svg-icon name="octicon-tag" :size="16" class="tw-mr-1"/>{{ textTags }}
        </button>
      </div>
      <div class="dropdown-divider branch-tag-divider"/>
      <div class="branch-tag-list" ref="elScrollContainer">
        <svg-icon name="octicon-rss" symbol-id="svg-symbol-octicon-rss"/>
        <div class="loading-indicator is-loading" v-if="tabLoadingStates[selectedTab] === 'loading'"/>
        <div v-for="(item, index) in filteredItems" :key="item.refShortName" class="dropdown-item" role="menuitem" :class="{active: item.selected}" :data-highlighted="activeItemIndex === index ? '' : undefined" @click="selectItem(item)">
          <span class="gt-ellipsis">{{ item.refShortName }}&nbsp;</span>
          <div class="ui label tw-shrink-0" v-if="item.refType === 'branch' && item.refShortName === currentRepoDefaultBranch">
            {{ textDefaultBranchLabel }}
          </div>
          <a v-if="enableFeed && selectedTab === 'branches'" role="button" class="rss-icon" target="_blank" @click.stop :href="item.rssFeedLink">
            <!-- creating a lot of Vue component is pretty slow, so we use a static SVG here -->
            <svg width="14" height="14" class="svg octicon-rss"><use href="#svg-symbol-octicon-rss"/></svg>
          </a>
        </div>
        <div class="dropdown-item tw-flex-col tw-items-start tw-gap-1" role="menuitem" v-if="showCreateNewRef" :data-highlighted="activeItemIndex === filteredItems.length ? '' : undefined" @click="createNewRef()">
          <div class="flex-text-block">
            <svg-icon :name="selectedTab === 'tags' ? 'octicon-tag' : 'octicon-git-branch'"/>
            <span v-text="trString(selectedTab === 'tags' ? textCreateTag : textCreateBranch, searchTerm)"/>
          </div>
          <div class="tw-text-xs">
            {{ textCreateRefFrom.replace('%s', currentRefShortName) }}
          </div>
          <form ref="elCreateNewRefForm" method="post" :action="createNewRefFormActionUrl">
            <input type="hidden" name="new_branch_name" :value="searchTerm">
            <input type="hidden" name="create_tag" :value="String(selectedTab === 'tags')">
            <input type="hidden" name="current_path" :value="currentTreePath">
          </form>
        </div>
      </div>
      <div class="dropdown-message" v-if="showNoResults">
        {{ textNoResults }}
      </div>
      <template v-if="showViewAllRefsEntry">
        <div class="dropdown-divider tw-m-0"/>
        <a v-if="selectedTab === 'branches'" class="dropdown-item" role="menuitem" :href="currentRepoLink + '/branches'">{{ textViewAllBranches }}</a>
        <a v-if="selectedTab === 'tags'" class="dropdown-item" role="menuitem" :href="currentRepoLink + '/tags'">{{ textViewAllTags }}</a>
      </template>
    </div>
  </div>
</template>
