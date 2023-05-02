<template>
  <div class="ui floating filter dropdown custom">
    <button class="branch-dropdown-button gt-ellipsis ui basic small compact button gt-df" @click="menuVisible = !menuVisible" @keyup.enter="menuVisible = !menuVisible">
      <span class="text gt-df gt-ac gt-mr-2">
        <template v-if="release">{{ textReleaseCompare }}</template>
        <template v-else>
          <svg-icon v-if="isViewTag" name="octicon-tag" />
          <svg-icon v-else name="octicon-git-branch"/>
          <strong ref="dropdownRefName" class="gt-ml-3">{{ refNameText }}</strong>
        </template>
      </span>
      <svg-icon name="octicon-triangle-down" :size="14" class-name="dropdown icon"/>
    </button>
    <div class="menu transition" :class="{visible: menuVisible}" v-if="menuVisible" v-cloak>
      <div class="ui icon search input">
        <i class="icon gt-df gt-ac gt-jc gt-m-0"><svg-icon name="octicon-filter" :size="16"/></i>
        <input name="search" ref="searchField" autocomplete="off" v-model="searchTerm" @keydown="keydown($event)" :placeholder="searchFieldPlaceholder">
      </div>
      <template v-if="showBranchesInDropdown">
        <div class="header branch-tag-choice">
          <div class="ui grid">
            <div class="two column row">
              <a class="reference column" href="#" @click="createTag = false; mode = 'branches'; focusSearchField()">
                <span class="text" :class="{black: mode === 'branches'}">
                  <svg-icon name="octicon-git-branch" :size="16" class-name="gt-mr-2"/>{{ textBranches }}
                </span>
              </a>
              <template v-if="!noTag">
                <a class="reference column" href="#" @click="createTag = true; mode = 'tags'; focusSearchField()">
                  <span class="text" :class="{black: mode === 'tags'}">
                    <svg-icon name="octicon-tag" :size="16" class-name="gt-mr-2"/>{{ textTags }}
                  </span>
                </a>
              </template>
            </div>
          </div>
        </div>
      </template>
      <div class="scrolling menu" ref="scrollContainer">
        <div v-for="(item, index) in filteredItems" :key="item.name" class="item" :class="{selected: item.selected, active: active === index}" @click="selectItem(item)" :ref="'listItem' + index">
          {{ item.name }}
          <a v-if="enableFeed && mode === 'branches'" role="button" class="ui compact muted right" :href="rssURLPrefix + item.url" target="_blank" @click.stop>
            <svg-icon name="octicon-rss" :size="14"/>
          </a>
        </div>
        <div class="item" v-if="showCreateNewBranch" :class="{active: active === filteredItems.length}" :ref="'listItem' + filteredItems.length">
          <a href="#" @click="createNewBranch()">
            <div v-show="createTag">
              <i class="reference tags icon"/>
              <!-- eslint-disable-next-line vue/no-v-html -->
              <span v-html="textCreateTag.replace('%s', searchTerm)"/>
            </div>
            <div v-show="!createTag">
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
            <input type="hidden" name="create_tag" v-model="createTag">
            <input type="hidden" name="current_path" v-model="treePath" v-if="treePath">
          </form>
        </div>
      </div>
      <div class="message" v-if="showNoResults">
        {{ noResults }}
      </div>
    </div>
  </div>
</template>

<script>
import {createApp, nextTick} from 'vue';
import $ from 'jquery';
import {SvgIcon} from '../svg.js';
import {pathEscapeSegments} from '../utils/url.js';

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
      this.active = (items.length === 0 && this.showCreateNewBranch ? 0 : -1);
      return items;
    },
    showNoResults() {
      return this.filteredItems.length === 0 && !this.showCreateNewBranch;
    },
    showCreateNewBranch() {
      if (this.disableCreateBranch || !this.searchTerm) {
        return false;
      }
      return this.items.filter((item) => item.name.toLowerCase() === this.searchTerm.toLowerCase()).length === 0;
    },
    formActionUrl() {
      return `${this.repoLink}/branches/_new/${pathEscapeSegments(this.branchNameSubURL)}`;
    },
  },

  watch: {
    menuVisible(visible) {
      if (visible) {
        this.focusSearchField();
      }
    }
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
          $(`#${this.branchForm}`).attr('action', url);
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
      let el = this.$refs[`listItem${this.active}`];
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
    }
  }
};

export function initRepoBranchTagSelector(selector) {
  for (const [elIndex, elRoot] of document.querySelectorAll(selector).entries()) {
    const data = {
      csrfToken: window.config.csrfToken,
      items: [],
      searchTerm: '',
      refNameText: '',
      menuVisible: false,
      createTag: false,
      release: null,

      isViewTag: false,
      isViewBranch: false,
      isViewTree: false,

      active: 0,

      ...window.config.pageData.branchDropdownDataList[elIndex],
    };

    // the "data.defaultBranch" is ambiguous, it could be "branch name" or "tag name"

    if (data.showBranchesInDropdown && data.branches) {
      for (const branch of data.branches) {
        data.items.push({name: branch, url: branch, branch: true, tag: false, selected: branch === data.defaultBranch});
      }
    }
    if (!data.noTag && data.tags) {
      for (const tag of data.tags) {
        if (data.release) {
          data.items.push({name: tag, url: tag, branch: false, tag: true, selected: tag === data.release.tagName});
        } else {
          data.items.push({name: tag, url: tag, branch: false, tag: true, selected: tag === data.defaultBranch});
        }
      }
    }

    const comp = {...sfc, data() { return data }};
    createApp(comp).mount(elRoot);
  }
}

export default sfc; // activate IDE's Vue plugin
</script>

<style scoped>
.menu .item a {
  display: none; /* only show RSS icon on hover */
}
.menu .item:hover a {
  display: inline-block;
}
</style>
