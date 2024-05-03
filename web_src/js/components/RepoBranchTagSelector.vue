<script>
import {createApp, nextTick} from 'vue';
import $ from 'jquery';
import {SvgIcon} from '../svg.js';
import {pathEscapeSegments} from '../utils/url.js';
import {showErrorToast} from '../modules/toast.js';
import {GET} from '../modules/fetch.js';

const sfc = {
  components: {SvgIcon},

  // no `data()`, at the moment, the `data()` is provided by the init code, which is not ideal and should be fixed in the future

  computed: {
    filteredItems() {
      const items = this.items.filter((item) => {
        return ((this.mode === 'branches' && item.branch) || (this.mode === 'tags' && item.tag)) &&
          (!this.searchTerm || item.name.toLowerCase().includes(this.searchTerm.toLowerCase()));
      });

      // TODO: fix this anti-pattern: side-effects-in-computed-properties
      this.active = !items.length && this.showCreateNewBranch ? 0 : -1;
      return items;
    },
    showNoResults() {
      return !this.filteredItems.length && !this.showCreateNewBranch;
    },
    showCreateNewBranch() {
      if (this.disableCreateBranch || !this.searchTerm) {
        return false;
      }
      return !this.items.filter((item) => {
        return item.name.toLowerCase() === this.searchTerm.toLowerCase();
      }).length;
    },
    formActionUrl() {
      return `${this.repoLink}/branches/_new/${this.branchNameSubURL}`;
    },
    shouldCreateTag() {
      return this.mode === 'tags';
    },
  },

  watch: {
    menuVisible(visible) {
      if (visible) {
        this.focusSearchField();
        this.fetchBranchesOrTags();
      }
    },
  },

  beforeMount() {
    if (this.viewType === 'tree') {
      this.isViewTree = true;
      this.refNameText = this.commitIdShort;
    } else if (this.viewType === 'tag') {
      this.isViewTag = true;
      this.refNameText = this.tagName;
    } else {
      this.isViewBranch = true;
      this.refNameText = this.branchName;
    }

    document.body.addEventListener('click', (event) => {
      if (this.$el.contains(event.target)) return;
      if (this.menuVisible) {
        this.menuVisible = false;
      }
    });
  },
  methods: {
    selectItem(item) {
      const prev = this.getSelected();
      if (prev !== null) {
        prev.selected = false;
      }
      item.selected = true;
      const url = (item.tag) ? this.tagURLPrefix + item.url + this.tagURLSuffix : this.branchURLPrefix + item.url + this.branchURLSuffix;
      if (!this.branchForm) {
        window.location.href = url;
      } else {
        this.isViewTree = false;
        this.isViewTag = false;
        this.isViewBranch = false;
        this.$refs.dropdownRefName.textContent = item.name;
        if (this.setAction) {
          document.getElementById(this.branchForm)?.setAttribute('action', url);
        } else {
          $(`#${this.branchForm} input[name="refURL"]`).val(url);
        }
        $(`#${this.branchForm} input[name="ref"]`).val(item.name);
        if (item.tag) {
          this.isViewTag = true;
          $(`#${this.branchForm} input[name="refType"]`).val('tag');
        } else {
          this.isViewBranch = true;
          $(`#${this.branchForm} input[name="refType"]`).val('branch');
        }
        if (this.submitForm) {
          $(`#${this.branchForm}`).trigger('submit');
        }
        this.menuVisible = false;
      }
    },
    createNewBranch() {
      if (!this.showCreateNewBranch) return;
      $(this.$refs.newBranchForm).trigger('submit');
    },
    focusSearchField() {
      nextTick(() => {
        this.$refs.searchField.focus();
      });
    },
    getSelected() {
      for (let i = 0, j = this.items.length; i < j; ++i) {
        if (this.items[i].selected) return this.items[i];
      }
      return null;
    },
    getSelectedIndexInFiltered() {
      for (let i = 0, j = this.filteredItems.length; i < j; ++i) {
        if (this.filteredItems[i].selected) return i;
      }
      return -1;
    },
    scrollToActive() {
      let el = this.$refs[`listItem${this.active}`]; // eslint-disable-line no-jquery/variable-pattern
      if (!el || !el.length) return;
      if (Array.isArray(el)) {
        el = el[0];
      }

      const cont = this.$refs.scrollContainer;
      if (el.offsetTop < cont.scrollTop) {
        cont.scrollTop = el.offsetTop;
      } else if (el.offsetTop + el.clientHeight > cont.scrollTop + cont.clientHeight) {
        cont.scrollTop = el.offsetTop + el.clientHeight - cont.clientHeight;
      }
    },
    keydown(event) {
      if (event.keyCode === 40) { // arrow down
        event.preventDefault();

        if (this.active === -1) {
          this.active = this.getSelectedIndexInFiltered();
        }

        if (this.active + (this.showCreateNewBranch ? 0 : 1) >= this.filteredItems.length) {
          return;
        }
        this.active++;
        this.scrollToActive();
      } else if (event.keyCode === 38) { // arrow up
        event.preventDefault();

        if (this.active === -1) {
          this.active = this.getSelectedIndexInFiltered();
        }

        if (this.active <= 0) {
          return;
        }
        this.active--;
        this.scrollToActive();
      } else if (event.keyCode === 13) { // enter
        event.preventDefault();

        if (this.active >= this.filteredItems.length) {
          this.createNewBranch();
        } else if (this.active >= 0) {
          this.selectItem(this.filteredItems[this.active]);
        }
      } else if (event.keyCode === 27) { // escape
        event.preventDefault();
        this.menuVisible = false;
      }
    },
    handleTabSwitch(mode) {
      if (this.isLoading) return;
      this.mode = mode;
      this.focusSearchField();
      this.fetchBranchesOrTags();
    },
    async fetchBranchesOrTags() {
      if (!['branches', 'tags'].includes(this.mode) || this.isLoading) return;
      // only fetch when branch/tag list has not been initialized
      if (this.hasListInitialized[this.mode] ||
        (this.mode === 'branches' && !this.showBranchesInDropdown) ||
        (this.mode === 'tags' && this.noTag)
      ) {
        return;
      }
      this.isLoading = true;
      try {
        const resp = await GET(`${this.repoLink}/${this.mode}/list`);
        const {results} = await resp.json();
        for (const result of results) {
          let selected = false;
          if (this.mode === 'branches') {
            selected = result === this.defaultSelectedRefName;
          } else {
            selected = result === (this.release ? this.release.tagName : this.defaultSelectedRefName);
          }
          this.items.push({name: result, url: pathEscapeSegments(result), branch: this.mode === 'branches', tag: this.mode === 'tags', selected});
        }
        this.hasListInitialized[this.mode] = true;
      } catch (e) {
        showErrorToast(`Network error when fetching ${this.mode}, error: ${e}`);
      } finally {
        this.isLoading = false;
      }
    },
  },
};

export function initRepoBranchTagSelector(selector) {
  for (const [elIndex, elRoot] of document.querySelectorAll(selector).entries()) {
    const data = {
      csrfToken: window.config.csrfToken,
      items: [],
      searchTerm: '',
      refNameText: '',
      menuVisible: false,
      release: null,

      isViewTag: false,
      isViewBranch: false,
      isViewTree: false,

      active: 0,
      isLoading: false,
      // This means whether branch list/tag list has initialized
      hasListInitialized: {
        'branches': false,
        'tags': false,
      },
      ...window.config.pageData.branchDropdownDataList[elIndex],
    };

    const comp = {...sfc, data() { return data }};
    createApp(comp).mount(elRoot);
  }
}

export default sfc; // activate IDE's Vue plugin
</script>
<template>
  <div class="ui dropdown custom branch-selector-dropdown">
    <div class="ui button branch-dropdown-button" @click="menuVisible = !menuVisible" @keyup.enter="menuVisible = !menuVisible">
      <span class="flex-text-block gt-ellipsis">
        <template v-if="release">{{ textReleaseCompare }}</template>
        <template v-else>
          <svg-icon v-if="isViewTag" name="octicon-tag"/>
          <svg-icon v-else name="octicon-git-branch"/>
          <strong ref="dropdownRefName" class="tw-ml-2 tw-inline-block gt-ellipsis">{{ refNameText }}</strong>
        </template>
      </span>
      <svg-icon name="octicon-triangle-down" :size="14" class-name="dropdown icon"/>
    </div>
    <div class="menu transition" :class="{visible: menuVisible}" v-show="menuVisible" v-cloak>
      <div class="ui icon search input">
        <i class="icon"><svg-icon name="octicon-filter" :size="16"/></i>
        <input name="search" ref="searchField" autocomplete="off" v-model="searchTerm" @keydown="keydown($event)" :placeholder="searchFieldPlaceholder">
      </div>
      <div v-if="showBranchesInDropdown" class="branch-tag-tab">
        <a class="branch-tag-item muted" :class="{active: mode === 'branches'}" href="#" @click="handleTabSwitch('branches')">
          <svg-icon name="octicon-git-branch" :size="16" class-name="tw-mr-1"/>{{ textBranches }}
        </a>
        <a v-if="!noTag" class="branch-tag-item muted" :class="{active: mode === 'tags'}" href="#" @click="handleTabSwitch('tags')">
          <svg-icon name="octicon-tag" :size="16" class-name="tw-mr-1"/>{{ textTags }}
        </a>
      </div>
      <div class="branch-tag-divider"/>
      <div class="scrolling menu" ref="scrollContainer">
        <svg-icon name="octicon-rss" symbol-id="svg-symbol-octicon-rss"/>
        <div class="loading-indicator is-loading" v-if="isLoading"/>
        <div v-for="(item, index) in filteredItems" :key="item.name" class="item" :class="{selected: item.selected, active: active === index}" @click="selectItem(item)" :ref="'listItem' + index">
          {{ item.name }}
          <div class="ui label" v-if="item.name===repoDefaultBranch && mode === 'branches'">
            {{ textDefaultBranchLabel }}
          </div>
          <a v-show="enableFeed && mode === 'branches'" role="button" class="rss-icon tw-float-right" :href="rssURLPrefix + item.url" target="_blank" @click.stop>
            <!-- creating a lot of Vue component is pretty slow, so we use a static SVG here -->
            <svg width="14" height="14" class="svg octicon-rss"><use href="#svg-symbol-octicon-rss"/></svg>
          </a>
        </div>
        <div class="item" v-if="showCreateNewBranch" :class="{active: active === filteredItems.length}" :ref="'listItem' + filteredItems.length">
          <a href="#" @click="createNewBranch()">
            <div v-show="shouldCreateTag">
              <i class="reference tags icon"/>
              <!-- eslint-disable-next-line vue/no-v-html -->
              <span v-html="textCreateTag.replace('%s', searchTerm)"/>
            </div>
            <div v-show="!shouldCreateTag">
              <svg-icon name="octicon-git-branch"/>
              <!-- eslint-disable-next-line vue/no-v-html -->
              <span v-html="textCreateBranch.replace('%s', searchTerm)"/>
            </div>
            <div class="text small">
              <span v-if="isViewBranch || release">{{ textCreateBranchFrom.replace('%s', branchName) }}</span>
              <span v-else-if="isViewTag">{{ textCreateBranchFrom.replace('%s', tagName) }}</span>
              <span v-else>{{ textCreateBranchFrom.replace('%s', commitIdShort) }}</span>
            </div>
          </a>
          <form ref="newBranchForm" :action="formActionUrl" method="post">
            <input type="hidden" name="_csrf" :value="csrfToken">
            <input type="hidden" name="new_branch_name" v-model="searchTerm">
            <input type="hidden" name="create_tag" v-model="shouldCreateTag">
            <input type="hidden" name="current_path" v-model="treePath" v-if="treePath">
          </form>
        </div>
      </div>
      <div class="message" v-if="showNoResults && !isLoading">
        {{ noResults }}
      </div>
    </div>
  </div>
</template>
