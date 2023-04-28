<template>
  <div>
    <div v-if="!isOrganization" class="ui two item menu">
      <a :class="{item: true, active: tab === 'repos'}" @click="changeTab('repos')">{{ textRepository }}</a>
      <a :class="{item: true, active: tab === 'organizations'}" @click="changeTab('organizations')">{{ textOrganization }}</a>
    </div>
    <div v-show="tab === 'repos'" class="ui tab active list dashboard-repos">
      <h4 class="ui top attached header gt-df gt-ac">
        <div class="gt-f1 gt-df gt-ac">
          {{ textMyRepos }}
          <span class="ui grey label gt-ml-3">{{ reposTotalCount }}</span>
        </div>
        <a :href="subUrl + '/repo/create' + (isOrganization ? '?org=' + organizationId : '')" :data-tooltip-content="textNewRepo">
          <svg-icon name="octicon-plus"/>
          <span class="sr-only">{{ textNewRepo }}</span>
        </a>
      </h4>
      <div class="ui attached segment repos-search">
        <div class="ui fluid right action left icon input" :class="{loading: isLoading}">
          <input @input="changeReposFilter(reposFilter)" v-model="searchQuery" ref="search" :placeholder="textSearchRepos">
          <i class="icon gt-df gt-ac gt-jc"><svg-icon name="octicon-search" :size="16"/></i>
          <div class="ui dropdown icon button" :title="textFilter">
            <i class="icon gt-df gt-ac gt-jc gt-m-0"><svg-icon name="octicon-filter" :size="16"/></i>
            <div class="menu">
              <a class="item" @click="toggleArchivedFilter()">
                <div class="ui checkbox" ref="checkboxArchivedFilter" :title="checkboxArchivedFilterTitle">
                  <!--the "hidden" is necessary to make the checkbox work without Fomantic UI js,
                      otherwise if the "input" handles click event for intermediate status, it breaks the internal state-->
                  <input type="checkbox" class="hidden" v-bind.prop="checkboxArchivedFilterProps">
                  <label>
                    <svg-icon name="octicon-archive" :size="16" class-name="gt-mr-2"/>
                    {{ textShowArchived }}
                  </label>
                </div>
              </a>
              <a class="item" @click="togglePrivateFilter()">
                <div class="ui checkbox" ref="checkboxPrivateFilter" :title="checkboxPrivateFilterTitle">
                  <input type="checkbox" class="hidden" v-bind.prop="checkboxPrivateFilterProps">
                  <label>
                    <svg-icon name="octicon-lock" :size="16" class-name="gt-mr-2"/>
                    {{ textShowPrivate }}
                  </label>
                </div>
              </a>
            </div>
          </div>
        </div>
        <div class="ui secondary tiny pointing borderless menu center grid repos-filter">
          <a class="item" :class="{active: reposFilter === 'all'}" @click="changeReposFilter('all')">
            {{ textAll }}
            <div v-show="reposFilter === 'all'" class="ui circular mini grey label">{{ repoTypeCount }}</div>
          </a>
          <a class="item" :class="{active: reposFilter === 'sources'}" @click="changeReposFilter('sources')">
            {{ textSources }}
            <div v-show="reposFilter === 'sources'" class="ui circular mini grey label">{{ repoTypeCount }}</div>
          </a>
          <a class="item" :class="{active: reposFilter === 'forks'}" @click="changeReposFilter('forks')">
            {{ textForks }}
            <div v-show="reposFilter === 'forks'" class="ui circular mini grey label">{{ repoTypeCount }}</div>
          </a>
          <a class="item" :class="{active: reposFilter === 'mirrors'}" @click="changeReposFilter('mirrors')" v-if="isMirrorsEnabled">
            {{ textMirrors }}
            <div v-show="reposFilter === 'mirrors'" class="ui circular mini grey label">{{ repoTypeCount }}</div>
          </a>
          <a class="item" :class="{active: reposFilter === 'collaborative'}" @click="changeReposFilter('collaborative')">
            {{ textCollaborative }}
            <div v-show="reposFilter === 'collaborative'" class="ui circular mini grey label">{{ repoTypeCount }}</div>
          </a>
        </div>
      </div>
      <div v-if="repos.length" class="ui attached table segment gt-rounded-bottom">
        <ul class="repo-owner-name-list">
          <li v-for="repo in repos" :class="{'private': repo.private || repo.internal}" :key="repo.id">
            <a class="repo-list-link gt-df gt-ac gt-sb" :href="repo.link">
              <div class="item-name gt-df gt-ac gt-f1">
                <svg-icon :name="repoIcon(repo)" :size="16" class-name="gt-mr-2"/>
                <div class="text gt-bold truncate gt-ml-1">{{ repo.full_name }}</div>
                <span v-if="repo.archived">
                  <svg-icon name="octicon-archive" :size="16" class-name="gt-ml-2"/>
                </span>
              </div>
            </a>
          </li>
        </ul>
        <div v-if="showMoreReposLink" class="center gt-py-3 gt-border-secondary-top">
          <div class="ui borderless pagination menu narrow">
            <a
              class="item navigation gt-py-2" :class="{'disabled': page === 1}"
              @click="changePage(1)" :title="textFirstPage"
            >
              <svg-icon name="gitea-double-chevron-left" :size="16" class-name="gt-mr-2"/>
            </a>
            <a
              class="item navigation gt-py-2" :class="{'disabled': page === 1}"
              @click="changePage(page - 1)" :title="textPreviousPage"
            >
              <svg-icon name="octicon-chevron-left" :size="16" clsas-name="gt-mr-2"/>
            </a>
            <a class="active item gt-py-2">{{ page }}</a>
            <a
              class="item navigation" :class="{'disabled': page === finalPage}"
              @click="changePage(page + 1)" :title="textNextPage"
            >
              <svg-icon name="octicon-chevron-right" :size="16" class-name="gt-ml-2"/>
            </a>
            <a
              class="item navigation gt-py-2" :class="{'disabled': page === finalPage}"
              @click="changePage(finalPage)" :title="textLastPage"
            >
              <svg-icon name="gitea-double-chevron-right" :size="16" class-name="gt-ml-2"/>
            </a>
          </div>
        </div>
      </div>
    </div>
    <div v-if="!isOrganization" v-show="tab === 'organizations'" class="ui tab active list dashboard-orgs">
      <h4 class="ui top attached header gt-df gt-ac">
        <div class="gt-f1 gt-df gt-ac">
          {{ textMyOrgs }}
          <span class="ui grey label gt-ml-3">{{ organizationsTotalCount }}</span>
        </div>
        <a v-if="canCreateOrganization" :href="subUrl + '/org/create'" :data-tooltip-content="textNewOrg">
          <svg-icon name="octicon-plus"/>
          <span class="sr-only">{{ textNewOrg }}</span>
        </a>
      </h4>
      <div v-if="organizations.length" class="ui attached table segment gt-rounded-bottom">
        <ul class="repo-owner-name-list">
          <li v-for="org in organizations" :key="org.name">
            <a class="repo-list-link gt-df gt-ac gt-sb" :href="subUrl + '/' + encodeURIComponent(org.name)">
              <div class="text truncate item-name gt-f1">
                <svg-icon name="octicon-organization" :size="16" class-name="gt-mr-2"/>
                <strong>{{ org.name }}</strong>
              </div>
              <div class="text light grey gt-df gt-ac">
                {{ org.num_repos }}
                <svg-icon name="octicon-repo" :size="16" class-name="gt-ml-2 gt-mt-1"/>
              </div>
            </a>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>

<script>
import {createApp, nextTick} from 'vue';
import $ from 'jquery';
import {SvgIcon} from '../svg.js';

const {appSubUrl, assetUrlPrefix, pageData} = window.config;

const sfc = {
  components: {SvgIcon},
  data() {
    const params = new URLSearchParams(window.location.search);
    const tab = params.get('repo-search-tab') || 'repos';
    const reposFilter = params.get('repo-search-filter') || 'all';
    const privateFilter = params.get('repo-search-private') || 'both';
    const archivedFilter = params.get('repo-search-archived') || 'unarchived';
    const searchQuery = params.get('repo-search-query') || '';
    const page = Number(params.get('repo-search-page')) || 1;

    return {
      tab,
      repos: [],
      reposTotalCount: 0,
      reposFilter,
      archivedFilter,
      privateFilter,
      page,
      finalPage: 1,
      searchQuery,
      isLoading: false,
      staticPrefix: assetUrlPrefix,
      counts: {},
      repoTypes: {
        all: {
          searchMode: '',
        },
        forks: {
          searchMode: 'fork',
        },
        mirrors: {
          searchMode: 'mirror',
        },
        sources: {
          searchMode: 'source',
        },
        collaborative: {
          searchMode: 'collaborative',
        },
      },
      textArchivedFilterTitles: {},
      textPrivateFilterTitles: {},

      organizations: [],
      isOrganization: true,
      canCreateOrganization: false,
      organizationsTotalCount: 0,
      organizationId: 0,

      subUrl: appSubUrl,
      ...pageData.dashboardRepoList,
    };
  },

  computed: {
    showMoreReposLink() {
      return this.repos.length > 0 && this.repos.length < this.counts[`${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`];
    },
    searchURL() {
      return `${this.subUrl}/repo/search?sort=updated&order=desc&uid=${this.uid}&team_id=${this.teamId}&q=${this.searchQuery
      }&page=${this.page}&limit=${this.searchLimit}&mode=${this.repoTypes[this.reposFilter].searchMode
      }${this.reposFilter !== 'all' ? '&exclusive=1' : ''
      }${this.archivedFilter === 'archived' ? '&archived=true' : ''}${this.archivedFilter === 'unarchived' ? '&archived=false' : ''
      }${this.privateFilter === 'private' ? '&is_private=true' : ''}${this.privateFilter === 'public' ? '&is_private=false' : ''
      }`;
    },
    repoTypeCount() {
      return this.counts[`${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`];
    },
    checkboxArchivedFilterTitle() {
      return this.textArchivedFilterTitles[this.archivedFilter];
    },
    checkboxArchivedFilterProps() {
      return {checked: this.archivedFilter === 'archived', indeterminate: this.archivedFilter === 'both'};
    },
    checkboxPrivateFilterTitle() {
      return this.textPrivateFilterTitles[this.privateFilter];
    },
    checkboxPrivateFilterProps() {
      return {checked: this.privateFilter === 'private', indeterminate: this.privateFilter === 'both'};
    },
  },

  mounted() {
    const el = document.getElementById('dashboard-repo-list');
    this.changeReposFilter(this.reposFilter);
    $(el).find('.dropdown').dropdown();
    nextTick(() => {
      this.$refs.search.focus();
    });

    this.textArchivedFilterTitles = {
      'archived': this.textShowOnlyArchived,
      'unarchived': this.textShowOnlyUnarchived,
      'both': this.textShowBothArchivedUnarchived,
    };

    this.textPrivateFilterTitles = {
      'private': this.textShowOnlyPrivate,
      'public': this.textShowOnlyPublic,
      'both': this.textShowBothPrivatePublic,
    };
  },

  methods: {
    changeTab(t) {
      this.tab = t;
      this.updateHistory();
    },

    changeReposFilter(filter) {
      this.reposFilter = filter;
      this.repos = [];
      this.page = 1;
      this.counts[`${filter}:${this.archivedFilter}:${this.privateFilter}`] = 0;
      this.searchRepos();
    },

    updateHistory() {
      const params = new URLSearchParams(window.location.search);

      if (this.tab === 'repos') {
        params.delete('repo-search-tab');
      } else {
        params.set('repo-search-tab', this.tab);
      }

      if (this.reposFilter === 'all') {
        params.delete('repo-search-filter');
      } else {
        params.set('repo-search-filter', this.reposFilter);
      }

      if (this.privateFilter === 'both') {
        params.delete('repo-search-private');
      } else {
        params.set('repo-search-private', this.privateFilter);
      }

      if (this.archivedFilter === 'unarchived') {
        params.delete('repo-search-archived');
      } else {
        params.set('repo-search-archived', this.archivedFilter);
      }

      if (this.searchQuery === '') {
        params.delete('repo-search-query');
      } else {
        params.set('repo-search-query', this.searchQuery);
      }

      if (this.page === 1) {
        params.delete('repo-search-page');
      } else {
        params.set('repo-search-page', `${this.page}`);
      }

      const queryString = params.toString();
      if (queryString) {
        window.history.replaceState({}, '', `?${queryString}`);
      } else {
        window.history.replaceState({}, '', window.location.pathname);
      }
    },

    toggleArchivedFilter() {
      if (this.archivedFilter === 'unarchived') {
        this.archivedFilter = 'archived';
      } else if (this.archivedFilter === 'archived') {
        this.archivedFilter = 'both';
      } else { // including both
        this.archivedFilter = 'unarchived';
      }
      this.page = 1;
      this.repos = [];
      this.counts[`${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`] = 0;
      this.searchRepos();
    },

    togglePrivateFilter() {
      if (this.privateFilter === 'both') {
        this.privateFilter = 'public';
      } else if (this.privateFilter === 'public') {
        this.privateFilter = 'private';
      } else { // including private
        this.privateFilter = 'both';
      }
      this.page = 1;
      this.repos = [];
      this.counts[`${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`] = 0;
      this.searchRepos();
    },


    changePage(page) {
      this.page = page;
      if (this.page > this.finalPage) {
        this.page = this.finalPage;
      }
      if (this.page < 1) {
        this.page = 1;
      }
      this.repos = [];
      this.counts[`${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`] = 0;
      this.searchRepos();
    },

    async searchRepos() {
      this.isLoading = true;

      const searchedMode = this.repoTypes[this.reposFilter].searchMode;
      const searchedURL = this.searchURL;
      const searchedQuery = this.searchQuery;

      let response, json;
      try {
        if (!this.reposTotalCount) {
          const totalCountSearchURL = `${this.subUrl}/repo/search?count_only=1&uid=${this.uid}&team_id=${this.teamId}&q=&page=1&mode=`;
          response = await fetch(totalCountSearchURL);
          this.reposTotalCount = response.headers.get('X-Total-Count');
        }

        response = await fetch(searchedURL);
        json = await response.json();
      } catch {
        if (searchedURL === this.searchURL) {
          this.isLoading = false;
        }
        return;
      }

      if (searchedURL === this.searchURL) {
        this.repos = json.data;
        const count = response.headers.get('X-Total-Count');
        if (searchedQuery === '' && searchedMode === '' && this.archivedFilter === 'both') {
          this.reposTotalCount = count;
        }
        this.counts[`${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`] = count;
        this.finalPage = Math.ceil(count / this.searchLimit);
        this.updateHistory();
        this.isLoading = false;
      }
    },

    repoIcon(repo) {
      if (repo.fork) {
        return 'octicon-repo-forked';
      } else if (repo.mirror) {
        return 'octicon-mirror';
      } else if (repo.template) {
        return `octicon-repo-template`;
      } else if (repo.private) {
        return 'octicon-lock';
      } else if (repo.internal) {
        return 'octicon-repo';
      }
      return 'octicon-repo';
    }
  },
};

export function initDashboardRepoList() {
  const el = document.getElementById('dashboard-repo-list');
  if (el) {
    createApp(sfc).mount(el);
  }
}

export default sfc; // activate the IDE's Vue plugin

</script>
