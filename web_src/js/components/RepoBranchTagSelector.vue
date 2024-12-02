<script lang="ts">
import {createApp, nextTick} from 'vue';
import {SvgIcon} from '../svg.ts';
import {showErrorToast} from '../modules/toast.ts';
import {GET} from '../modules/fetch.ts';
import {pathEscapeSegments} from '../utils/url.ts';
import type {GitRefType} from '../types.ts';

type ListItem = {
  selected: boolean;
  refShortName: string;
  refType: GitRefType;
  rssFeedLink: string;
};

type SelectedTab = 'branches' | 'tags';

type TabLoadingStates = Record<SelectedTab, '' | 'loading' | 'done'>

let currentElRoot: HTMLElement;

const sfc = {
  components: {SvgIcon},

  computed: {
    searchFieldPlaceholder() {
      return this.selectedTab === 'branches' ? this.textFilterBranch : this.textFilterTag;
    },
    filteredItems(): ListItem[] {
      const searchTermLower = this.searchTerm.toLowerCase();
      const items = this.allItems.filter((item: ListItem) => {
        const typeMatched = (this.selectedTab === 'branches' && item.refType === 'branch') || (this.selectedTab === 'tags' && item.refType === 'tag');
        if (!typeMatched) return false;
        if (!this.searchTerm) return true; // match all
        return item.refShortName.toLowerCase().includes(searchTermLower);
      });

      // TODO: fix this anti-pattern: side-effects-in-computed-properties
      this.activeItemIndex = !items.length && this.showCreateNewRef ? 0 : -1;
      return items;
    },
    showNoResults() {
      if (this.tabLoadingStates[this.selectedTab] !== 'done') return false;
      return !this.filteredItems.length && !this.showCreateNewRef;
    },
    showCreateNewRef() {
      if (!this.allowCreateNewRef || !this.searchTerm) {
        return false;
      }
      return !this.allItems.filter((item: ListItem) => {
        return item.refShortName === this.searchTerm; // FIXME: not quite right here, it mixes "branch" and "tag" names
      }).length;
    },
    createNewRefFormActionUrl() {
      return `${this.currentRepoLink}/branches/_new/${this.currentRefType}/${pathEscapeSegments(this.currentRefShortName)}`;
    },
  },

  watch: {
    menuVisible(visible) {
      if (!visible) return;
      this.focusSearchField();
      this.loadTabItems();
    },
  },

  data() {
    const elRoot = currentElRoot;
    const shouldShowTabBranches = elRoot.getAttribute('data-show-tab-branches') === 'true';
    return {
      csrfToken: window.config.csrfToken,
      allItems: [] as ListItem[],
      selectedTab: (shouldShowTabBranches ? 'branches' : 'tags') as SelectedTab,
      searchTerm: '',
      menuVisible: false,
      activeItemIndex: 0,
      tabLoadingStates: {} as TabLoadingStates,

      textReleaseCompare: elRoot.getAttribute('data-text-release-compare'),
      textBranches: elRoot.getAttribute('data-text-branches'),
      textTags: elRoot.getAttribute('data-text-tags'),
      textFilterBranch: elRoot.getAttribute('data-text-filter-branch'),
      textFilterTag: elRoot.getAttribute('data-text-filter-tag'),
      textDefaultBranchLabel: elRoot.getAttribute('data-text-default-branch-label'),
      textCreateTag: elRoot.getAttribute('data-text-create-tag'),
      textCreateBranch: elRoot.getAttribute('data-text-create-branch'),
      textCreateRefFrom: elRoot.getAttribute('data-text-create-ref-from'),
      textNoResults: elRoot.getAttribute('data-text-no-results'),

      currentRepoDefaultBranch: elRoot.getAttribute('data-current-repo-default-branch'),
      currentRepoLink: elRoot.getAttribute('data-current-repo-link'),
      currentTreePath: elRoot.getAttribute('data-current-tree-path'),
      currentRefType: elRoot.getAttribute('data-current-ref-type'),
      currentRefShortName: elRoot.getAttribute('data-current-ref-short-name'),

      refLinkTemplate: elRoot.getAttribute('data-ref-link-template'),
      refFormActionTemplate: elRoot.getAttribute('data-ref-form-action-template'),
      dropdownFixedText: elRoot.getAttribute('data-dropdown-fixed-text'),
      showTabBranches: shouldShowTabBranches,
      showTabTags: elRoot.getAttribute('data-show-tab-tags') === 'true',
      allowCreateNewRef: elRoot.getAttribute('data-allow-create-new-ref') === 'true',

      enableFeed: elRoot.getAttribute('data-enable-feed') === 'true',
    };
  },

  beforeMount() {
    document.body.addEventListener('click', (e) => {
      if (this.$el.contains(e.target)) return;
      if (this.menuVisible) this.menuVisible = false;
    });
  },
  methods: {
    selectItem(item: ListItem) {
      this.menuVisible = false;
      if (this.refFormActionTemplate) {
        this.currentRefType = item.refType;
        this.currentRefShortName = item.refShortName;
        let actionLink = this.refFormActionTemplate;
        actionLink = actionLink.replace('{RepoLink}', this.currentRepoLink);
        actionLink = actionLink.replace('{RefType}', pathEscapeSegments(item.refType));
        actionLink = actionLink.replace('{RefShortName}', pathEscapeSegments(item.refShortName));
        this.$el.closest('form').action = actionLink;
      } else {
        let link = this.refLinkTemplate;
        link = link.replace('{RepoLink}', this.currentRepoLink);
        link = link.replace('{RefType}', pathEscapeSegments(item.refType));
        link = link.replace('{RefShortName}', pathEscapeSegments(item.refShortName));
        link = link.replace('{TreePath}', pathEscapeSegments(this.currentTreePath));
        window.location.href = link;
      }
    },
    createNewRef() {
      this.$refs.createNewRefForm?.submit();
    },
    focusSearchField() {
      nextTick(() => {
        this.$refs.searchField.focus();
      });
    },
    getSelectedIndexInFiltered() {
      for (let i = 0; i < this.filteredItems.length; ++i) {
        if (this.filteredItems[i].selected) return i;
      }
      return -1;
    },
    getActiveItem() {
      const el = this.$refs[`listItem${this.activeItemIndex}`]; // eslint-disable-line no-jquery/variable-pattern
      return (el && el.length) ? el[0] : null;
    },
    keydown(e) {
      if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
        e.preventDefault();

        if (this.activeItemIndex === -1) {
          this.activeItemIndex = this.getSelectedIndexInFiltered();
        }
        const nextIndex = e.key === 'ArrowDown' ? this.activeItemIndex + 1 : this.activeItemIndex - 1;
        if (nextIndex < 0) {
          return;
        }
        if (nextIndex + (this.showCreateNewRef ? 0 : 1) > this.filteredItems.length) {
          return;
        }
        this.activeItemIndex = nextIndex;
        this.getActiveItem().scrollIntoView({block: 'nearest'});
      } else if (e.key === 'Enter') {
        e.preventDefault();
        this.getActiveItem()?.click();
      } else if (e.key === 'Escape') {
        e.preventDefault();
        this.menuVisible = false;
      }
    },
    handleTabSwitch(selectedTab) {
      this.selectedTab = selectedTab;
      this.focusSearchField();
      this.loadTabItems();
    },
    async loadTabItems() {
      const tab = this.selectedTab;
      if (this.tabLoadingStates[tab] === 'loading' || this.tabLoadingStates[tab] === 'done') return;

      const refType = this.selectedTab === 'branches' ? 'branch' : 'tag';
      this.tabLoadingStates[tab] = 'loading';
      try {
        const url = refType === 'branch' ? `${this.currentRepoLink}/branches/list` : `${this.currentRepoLink}/tags/list`;
        const resp = await GET(url);
        const {results} = await resp.json();
        for (const refShortName of results) {
          const item: ListItem = {
            refType,
            refShortName,
            selected: refType === this.currentRefType && refShortName === this.currentRefShortName,
            rssFeedLink: `${this.currentRepoLink}/rss/${refType}/${pathEscapeSegments(refShortName)}`,
          };
          this.allItems.push(item);
        }
        this.tabLoadingStates[tab] = 'done';
      } catch (e) {
        this.tabLoadingStates[tab] = '';
        showErrorToast(`Network error when fetching items for ${tab}, error: ${e}`);
        console.error(e);
      }
    },
  },
};

export function initRepoBranchTagSelector(selector) {
  for (const elRoot of document.querySelectorAll(selector)) {
    // it is very hacky, but it is the only way to pass the elRoot to the "data()" function
    // it could be improved in the future to do more rewriting.
    currentElRoot = elRoot;
    const comp = {...sfc};
    createApp(comp).mount(elRoot);
  }
}

export default sfc; // activate IDE's Vue plugin
</script>
<template>
  <div class="ui dropdown custom branch-selector-dropdown ellipsis-items-nowrap">
    <div tabindex="0" class="ui button branch-dropdown-button" @click="menuVisible = !menuVisible">
      <span class="flex-text-block gt-ellipsis">
        <template v-if="dropdownFixedText">{{ dropdownFixedText }}</template>
        <template v-else>
          <svg-icon v-if="currentRefType === 'tag'" name="octicon-tag"/>
          <svg-icon v-else name="octicon-git-branch"/>
          <strong ref="dropdownRefName" class="tw-ml-2 tw-inline-block gt-ellipsis">{{ currentRefShortName }}</strong>
        </template>
      </span>
      <svg-icon name="octicon-triangle-down" :size="14" class-name="dropdown icon"/>
    </div>
    <div class="menu transition" :class="{visible: menuVisible}" v-show="menuVisible" v-cloak>
      <div class="ui icon search input">
        <i class="icon"><svg-icon name="octicon-filter" :size="16"/></i>
        <input name="search" ref="searchField" autocomplete="off" v-model="searchTerm" @keydown="keydown($event)" :placeholder="searchFieldPlaceholder">
      </div>
      <div v-if="showTabBranches" class="branch-tag-tab">
        <a class="branch-tag-item muted" :class="{active: selectedTab === 'branches'}" href="#" @click="handleTabSwitch('branches')">
          <svg-icon name="octicon-git-branch" :size="16" class-name="tw-mr-1"/>{{ textBranches }}
        </a>
        <a v-if="showTabTags" class="branch-tag-item muted" :class="{active: selectedTab === 'tags'}" href="#" @click="handleTabSwitch('tags')">
          <svg-icon name="octicon-tag" :size="16" class-name="tw-mr-1"/>{{ textTags }}
        </a>
      </div>
      <div class="branch-tag-divider"/>
      <div class="scrolling menu" ref="scrollContainer">
        <svg-icon name="octicon-rss" symbol-id="svg-symbol-octicon-rss"/>
        <div class="loading-indicator is-loading" v-if="tabLoadingStates[selectedTab] === 'loading'"/>
        <div v-for="(item, index) in filteredItems" :key="item.refShortName" class="item" :class="{selected: item.selected, active: activeItemIndex === index}" @click="selectItem(item)" :ref="'listItem' + index">
          {{ item.refShortName }}
          <div class="ui label" v-if="item.refType === 'branch' && item.refShortName === currentRepoDefaultBranch">
            {{ textDefaultBranchLabel }}
          </div>
          <a v-if="enableFeed && selectedTab === 'branches'" role="button" class="rss-icon" target="_blank" @click.stop :href="item.rssFeedLink">
            <!-- creating a lot of Vue component is pretty slow, so we use a static SVG here -->
            <svg width="14" height="14" class="svg octicon-rss"><use href="#svg-symbol-octicon-rss"/></svg>
          </a>
        </div>
        <div class="item" v-if="showCreateNewRef" :class="{active: activeItemIndex === filteredItems.length}" :ref="'listItem' + filteredItems.length" @click="createNewRef()">
          <div v-if="selectedTab === 'tags'">
            <svg-icon name="octicon-tag" class="tw-mr-1"/>
            <span v-text="textCreateTag.replace('%s', searchTerm)"/>
          </div>
          <div v-else>
            <svg-icon name="octicon-git-branch" class="tw-mr-1"/>
            <span v-text="textCreateBranch.replace('%s', searchTerm)"/>
          </div>
          <div class="text small">
            {{ textCreateRefFrom.replace('%s', currentRefShortName) }}
          </div>
          <form ref="createNewRefForm" method="post" :action="createNewRefFormActionUrl">
            <input type="hidden" name="_csrf" :value="csrfToken">
            <input type="hidden" name="new_branch_name" :value="searchTerm">
            <input type="hidden" name="create_tag" :value="String(selectedTab === 'tags')">
            <input type="hidden" name="current_path" :value="currentTreePath">
          </form>
        </div>
      </div>
      <div class="message" v-if="showNoResults">
        {{ textNoResults }}
      </div>
    </div>
  </div>
</template>
