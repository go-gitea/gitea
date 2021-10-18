import Vue from 'vue';
import {initVueSvg, vueDelimiters} from './VueComponentLoader.js';

const {AppSubUrl, AssetUrlPrefix, pageData} = window.config;

function syncQueryParam(params, queryKey, queryValue, definedValue) {
  if (queryValue === definedValue) {
    params.delete(queryKey);
  } else {
    params.set(queryKey, queryValue);
  }
}

function refreshRepoSearch(repoSearch, page = 1) {
  repoSearch.page = page;
  repoSearch.repos = [];
  repoSearch.setCheckboxes();
  // If Vue is not notified manually, the computed property `repoTypeCount` doesn't get updated correctly
  // Leading to a not-rendered round counter initially
  Vue.set(repoSearch.counts, `${repoSearch.reposFilter}:${repoSearch.archivedFilter}:${repoSearch.privateFilter}`, 0);
  repoSearch.searchRepos();
}

function initVueComponents() {
  Vue.component('repo-search', {
    delimiters: vueDelimiters,
    props: {
      searchLimit: {
        type: Number,
        default: 10
      },
      subUrl: {
        type: String,
        required: true
      },
      uid: {
        type: Number,
        default: 0
      },
      teamId: {
        type: Number,
        required: false,
        default: 0
      },
      organizations: {
        type: Array,
        default: () => [],
      },
      isOrganization: {
        type: Boolean,
        default: true
      },
      canCreateOrganization: {
        type: Boolean,
        default: false
      },
      organizationsTotalCount: {
        type: Number,
        default: 0
      },
      moreReposLink: {
        type: String,
        default: ''
      }
    },

    data() {
      const params = new URLSearchParams(window.location.search);

      let tab = params.get('repo-search-tab');
      if (!tab) {
        tab = 'repos';
      }

      let reposFilter = params.get('repo-search-filter');
      if (!reposFilter) {
        reposFilter = 'all';
      }

      let privateFilter = params.get('repo-search-private');
      if (!privateFilter) {
        privateFilter = 'both';
      }

      let archivedFilter = params.get('repo-search-archived');
      if (!archivedFilter) {
        archivedFilter = 'unarchived';
      }

      let searchQuery = params.get('repo-search-query');
      if (!searchQuery) {
        searchQuery = '';
      }

      let page = 1;
      try {
        page = parseInt(params.get('repo-search-page'));
      } catch {
        // noop
      }
      if (!page) {
        page = 1;
      }

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
        staticPrefix: AssetUrlPrefix,
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
        }
      };
    },

    computed: {
      // used in `repolist.tmpl`
      showMoreReposLink() {
        return this.repos.length > 0 && this.repos.length < this.counts[`${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`];
      },
      searchURL() {
        return `${this.subUrl}/api/v1/repos/search?sort=updated&order=desc&uid=${this.uid}&team_id=${this.teamId}&q=${this.searchQuery
        }&page=${this.page}&limit=${this.searchLimit}&mode=${this.repoTypes[this.reposFilter].searchMode
        }${this.reposFilter !== 'all' ? '&exclusive=1' : ''
        }${this.archivedFilter === 'archived' ? '&archived=true' : ''}${this.archivedFilter === 'unarchived' ? '&archived=false' : ''
        }${this.privateFilter === 'private' ? '&is_private=true' : ''}${this.privateFilter === 'public' ? '&is_private=false' : ''
        }`;
      },
      repoTypeCount() {
        return this.counts[`${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`];
      }
    },

    mounted() {
      this.changeReposFilter(this.reposFilter);
      $(this.$el).find('.poping.up').popup();
      $(this.$el).find('.dropdown').dropdown();
      this.setCheckboxes();
      Vue.nextTick(() => {
        this.$refs.search.focus();
      });
    },

    methods: {
      changeTab(t) {
        this.tab = t;
        this.updateHistory();
      },

      setCheckboxes() {
        switch (this.archivedFilter) {
          case 'unarchived':
            $('#archivedFilterCheckbox').checkbox('set unchecked');
            break;
          case 'archived':
            $('#archivedFilterCheckbox').checkbox('set checked');
            break;
          case 'both':
            $('#archivedFilterCheckbox').checkbox('set indeterminate');
            break;
          default:
            this.archivedFilter = 'unarchived';
            $('#archivedFilterCheckbox').checkbox('set unchecked');
            break;
        }
        switch (this.privateFilter) {
          case 'public':
            $('#privateFilterCheckbox').checkbox('set unchecked');
            break;
          case 'private':
            $('#privateFilterCheckbox').checkbox('set checked');
            break;
          case 'both':
            $('#privateFilterCheckbox').checkbox('set indeterminate');
            break;
          default:
            this.privateFilter = 'both';
            $('#privateFilterCheckbox').checkbox('set indeterminate');
            break;
        }
      },

      changeReposFilter(filter) {
        this.reposFilter = filter;
        refreshRepoSearch(this);
      },

      updateHistory() {
        const params = new URLSearchParams(window.location.search);

        syncQueryParam(params, 'repo-search-tab', this.tab, 'repos');
        syncQueryParam(params, 'repo-search-filter', this.reposFilter, 'all');
        syncQueryParam(params, 'repo-search-private', this.privateFilter, 'both');
        syncQueryParam(params, 'repo-search-archived', this.archivedFilter, 'unarchived');
        syncQueryParam(params, 'repo-search-query', this.searchQuery, '');
        syncQueryParam(params, 'repo-search-page', this.page.toString(), '1');

        const queryString = params.toString();
        const newUrl = queryString ? `?${queryString}` : window.location.pathname;
        window.history.replaceState({}, '', newUrl);
      },

      toggleArchivedFilter() {
        switch (this.archivedFilter) {
          case 'unarchived':
            this.archivedFilter = 'archived';
            break;
          case 'archived':
            this.archivedFilter = 'both';
            break;
          case 'both':
          default:
            this.archivedFilter = 'unarchived';
            break;
        }
        refreshRepoSearch(this);
      },

      togglePrivateFilter() {
        switch (this.privateFilter) {
          case 'both':
            this.privateFilter = 'public';
            break;
          case 'public':
            this.privateFilter = 'private';
            break;
          case 'private':
          default:
            this.privateFilter = 'both';
            break;
        }
        refreshRepoSearch(this);
      },


      changePage(page) {
        refreshRepoSearch(this, Math.max(Math.min(page, this.finalPage), 1));
      },

      searchRepos() {
        this.isLoading = true;

        if (!this.reposTotalCount) {
          const totalCountSearchURL = `${this.subUrl}/api/v1/repos/search?sort=updated&order=desc&uid=${this.uid}&team_id=${this.teamId}&q=&page=1&mode=`;
          $.getJSON(totalCountSearchURL, (_result, _textStatus, request) => {
            this.reposTotalCount = request.getResponseHeader('X-Total-Count');
          });
        }

        const searchedMode = this.repoTypes[this.reposFilter].searchMode;
        const searchedURL = this.searchURL;
        const searchedQuery = this.searchQuery;

        $.getJSON(searchedURL, (result, _textStatus, request) => {
          if (searchedURL === this.searchURL) {
            this.repos = result.data;
            const count = request.getResponseHeader('X-Total-Count');
            if (searchedQuery === '' && searchedMode === '' && this.archivedFilter === 'both') {
              this.reposTotalCount = count;
            }
            Vue.set(this.counts, `${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`, count);
            this.finalPage = Math.ceil(count / this.searchLimit);
            this.updateHistory();
          }
        }).always(() => {
          if (searchedURL === this.searchURL) {
            this.isLoading = false;
          }
        });
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
    }
  });
}


export function initDashboardRepoList() {
  const el = document.getElementById('dashboard-repo-list');
  const dashboardRepoListData = pageData.dashboardRepoList || null;
  if (!el || !dashboardRepoListData) return;

  initVueSvg();
  initVueComponents();
  new Vue({
    el,
    delimiters: vueDelimiters,
    data: () => {
      return {
        searchLimit: dashboardRepoListData.searchLimit || 0,
        subUrl: AppSubUrl,
      };
    },
  });
}
