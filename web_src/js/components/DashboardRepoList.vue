<script lang="ts">
import {nextTick, defineComponent} from 'vue';
import {SvgIcon} from '../svg.ts';
import {GET} from '../modules/fetch.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

const {appSubUrl, assetUrlPrefix, pageData} = window.config;

type CommitStatus = 'pending' | 'success' | 'error' | 'failure' | 'warning';

type CommitStatusMap = {
  [status in CommitStatus]: {
    name: string,
    color: string,
  };
};

// make sure this matches templates/repo/commit_status.tmpl
const commitStatus: CommitStatusMap = {
  pending: {name: 'octicon-dot-fill', color: 'yellow'},
  success: {name: 'octicon-check', color: 'green'},
  error: {name: 'gitea-exclamation', color: 'red'},
  failure: {name: 'octicon-x', color: 'red'},
  warning: {name: 'gitea-exclamation', color: 'yellow'},
};

export default defineComponent({
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
      activeIndex: -1, // don't select anything at load, first cursor down will select
    };
  },

  computed: {
    showMoreReposLink() {
      return this.repos.length > 0 && this.repos.length < this.counts[`${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`];
    },
    searchURL() {
      return `${this.subUrl}/repo/search?sort=updated&order=desc&uid=${this.uid}&team_id=${this.teamId}&q=${this.searchQuery
      }&page=${this.page}&limit=${this.searchLimit}&mode=${this.repoTypes[this.reposFilter].searchMode
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
    const el = document.querySelector('#dashboard-repo-list');
    this.changeReposFilter(this.reposFilter);
    fomanticQuery(el.querySelector('.ui.dropdown')).dropdown();
    nextTick(() => {
      this.$refs.search?.focus();
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
    changeTab(tab: string) {
      this.tab = tab;
      this.updateHistory();
    },

    changeReposFilter(filter: string) {
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

    changePage(page: number) {
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
          response = await GET(totalCountSearchURL);
          this.reposTotalCount = parseInt(response.headers.get('X-Total-Count') ?? '0');
        }

        response = await GET(searchedURL);
        json = await response.json();
      } catch {
        if (searchedURL === this.searchURL) {
          this.isLoading = false;
        }
        return;
      }

      if (searchedURL === this.searchURL) {
        this.repos = json.data.map((webSearchRepo: any) => {
          return {
            ...webSearchRepo.repository,
            latest_commit_status_state: webSearchRepo.latest_commit_status?.State, // if latest_commit_status is null, it means there is no commit status
            latest_commit_status_state_link: webSearchRepo.latest_commit_status?.TargetURL,
            locale_latest_commit_status_state: webSearchRepo.locale_latest_commit_status,
          };
        });
        const count = Number(response.headers.get('X-Total-Count'));
        if (searchedQuery === '' && searchedMode === '' && this.archivedFilter === 'both') {
          this.reposTotalCount = count;
        }
        this.counts[`${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`] = count;
        this.finalPage = Math.ceil(count / this.searchLimit);
        this.updateHistory();
        this.isLoading = false;
      }
    },

    repoIcon(repo: any) {
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
    },

    statusIcon(status: CommitStatus) {
      return commitStatus[status].name;
    },

    statusColor(status: CommitStatus) {
      return commitStatus[status].color;
    },

    reposFilterKeyControl(e: KeyboardEvent) {
      switch (e.key) {
        case 'Enter':
          document.querySelector<HTMLAnchorElement>('.repo-owner-name-list li.active a')?.click();
          break;
        case 'ArrowUp':
          if (this.activeIndex > 0) {
            this.activeIndex--;
          } else if (this.page > 1) {
            this.changePage(this.page - 1);
            this.activeIndex = this.searchLimit - 1;
          }
          break;
        case 'ArrowDown':
          if (this.activeIndex < this.repos.length - 1) {
            this.activeIndex++;
          } else if (this.page < this.finalPage) {
            this.activeIndex = 0;
            this.changePage(this.page + 1);
          }
          break;
        case 'ArrowRight':
          if (this.page < this.finalPage) {
            this.changePage(this.page + 1);
          }
          break;
        case 'ArrowLeft':
          if (this.page > 1) {
            this.changePage(this.page - 1);
          }
          break;
      }
      if (this.activeIndex === -1 || this.activeIndex > this.repos.length - 1) {
        this.activeIndex = 0;
      }
    },
  },
});
</script>
<template>
  <div>
    <div v-if="!isOrganization" class="ui two item menu">
      <a :class="{item: true, active: tab === 'repos'}" @click="changeTab('repos')">{{ textRepository }}</a>
      <a :class="{item: true, active: tab === 'organizations'}" @click="changeTab('organizations')">{{ textOrganization }}</a>
    </div>
    <div v-show="tab === 'repos'" class="ui tab active list dashboard-repos">
      <h4 class="ui top attached header tw-flex tw-items-center">
        <div class="tw-flex-1 tw-flex tw-items-center">
          {{ textMyRepos }}
          <span class="ui grey label tw-ml-2">{{ reposTotalCount }}</span>
        </div>
        <a class="tw-flex tw-items-center muted" :href="subUrl + '/repo/create' + (isOrganization ? '?org=' + organizationId : '')" :data-tooltip-content="textNewRepo">
          <svg-icon name="octicon-plus"/>
        </a>
      </h4>
      <div v-if="!reposTotalCount" class="ui attached segment">
        <div v-if="!isLoading" class="empty-repo-or-org">
          <svg-icon name="octicon-git-branch" :size="24"/>
          <p>{{ textNoRepo }}</p>
        </div>
        <!-- using the loading indicator here will cause more (unnecessary) page flickers, so at the moment, not use the loading indicator -->
        <!-- <div v-else class="is-loading loading-icon-2px tw-min-h-16"/> -->
      </div>
      <div v-else class="ui attached segment repos-search">
        <div class="ui small fluid action left icon input">
          <input type="search" spellcheck="false" maxlength="255" @input="changeReposFilter(reposFilter)" v-model="searchQuery" ref="search" @keydown="reposFilterKeyControl" :placeholder="textSearchRepos">
          <i class="icon loading-icon-3px" :class="{'is-loading': isLoading}"><svg-icon name="octicon-search" :size="16"/></i>
          <div class="ui dropdown icon button" :title="textFilter">
            <svg-icon name="octicon-filter" :size="16"/>
            <div class="menu">
              <a class="item" @click="toggleArchivedFilter()">
                <div class="ui checkbox" ref="checkboxArchivedFilter" :title="checkboxArchivedFilterTitle">
                  <!--the "tw-pointer-events-none" is necessary to prevent the checkbox from handling user's input,
                      otherwise if the "input" handles click event for intermediate status, it breaks the internal state-->
                  <input type="checkbox" class="tw-pointer-events-none" v-bind.prop="checkboxArchivedFilterProps">
                  <label>
                    <svg-icon name="octicon-archive" :size="16" class="tw-mr-1"/>
                    {{ textShowArchived }}
                  </label>
                </div>
              </a>
              <a class="item" @click="togglePrivateFilter()">
                <div class="ui checkbox" ref="checkboxPrivateFilter" :title="checkboxPrivateFilterTitle">
                  <input type="checkbox" class="tw-pointer-events-none" v-bind.prop="checkboxPrivateFilterProps">
                  <label>
                    <svg-icon name="octicon-lock" :size="16" class="tw-mr-1"/>
                    {{ textShowPrivate }}
                  </label>
                </div>
              </a>
            </div>
          </div>
        </div>
        <overflow-menu class="ui secondary pointing tabular borderless menu repos-filter">
          <div class="overflow-menu-items tw-justify-center">
            <a class="item" tabindex="0" :class="{active: reposFilter === 'all'}" @click="changeReposFilter('all')">
              {{ textAll }}
              <div v-show="reposFilter === 'all'" class="ui circular mini grey label">{{ repoTypeCount }}</div>
            </a>
            <a class="item" tabindex="0" :class="{active: reposFilter === 'sources'}" @click="changeReposFilter('sources')">
              {{ textSources }}
              <div v-show="reposFilter === 'sources'" class="ui circular mini grey label">{{ repoTypeCount }}</div>
            </a>
            <a class="item" tabindex="0" :class="{active: reposFilter === 'forks'}" @click="changeReposFilter('forks')">
              {{ textForks }}
              <div v-show="reposFilter === 'forks'" class="ui circular mini grey label">{{ repoTypeCount }}</div>
            </a>
            <a class="item" tabindex="0" :class="{active: reposFilter === 'mirrors'}" @click="changeReposFilter('mirrors')" v-if="isMirrorsEnabled">
              {{ textMirrors }}
              <div v-show="reposFilter === 'mirrors'" class="ui circular mini grey label">{{ repoTypeCount }}</div>
            </a>
            <a class="item" tabindex="0" :class="{active: reposFilter === 'collaborative'}" @click="changeReposFilter('collaborative')">
              {{ textCollaborative }}
              <div v-show="reposFilter === 'collaborative'" class="ui circular mini grey label">{{ repoTypeCount }}</div>
            </a>
          </div>
        </overflow-menu>
      </div>
      <div v-if="repos.length" class="ui attached table segment tw-rounded-b">
        <ul class="repo-owner-name-list">
          <li class="tw-flex tw-items-center tw-py-2" v-for="repo, index in repos" :class="{'active': index === activeIndex}" :key="repo.id">
            <a class="repo-list-link muted" :href="repo.link">
              <svg-icon :name="repoIcon(repo)" :size="16" class="repo-list-icon"/>
              <div class="text truncate">{{ repo.full_name }}</div>
              <div v-if="repo.archived">
                <svg-icon name="octicon-archive" :size="16"/>
              </div>
            </a>
            <a class="tw-flex tw-items-center" v-if="repo.latest_commit_status_state" :href="repo.latest_commit_status_state_link" :data-tooltip-content="repo.locale_latest_commit_status_state">
              <!-- the commit status icon logic is taken from templates/repo/commit_status.tmpl -->
              <svg-icon :name="statusIcon(repo.latest_commit_status_state)" :class="'tw-ml-2 commit-status icon text ' + statusColor(repo.latest_commit_status_state)" :size="16"/>
            </a>
          </li>
        </ul>
        <div v-if="showMoreReposLink" class="tw-text-center">
          <div class="divider tw-my-0"/>
          <div class="ui borderless pagination menu narrow tw-my-2">
            <a
              class="item navigation tw-py-1" :class="{'disabled': page === 1}"
              @click="changePage(1)" :title="textFirstPage"
            >
              <svg-icon name="gitea-double-chevron-left" :size="16" class="tw-mr-1"/>
            </a>
            <a
              class="item navigation tw-py-1" :class="{'disabled': page === 1}"
              @click="changePage(page - 1)" :title="textPreviousPage"
            >
              <svg-icon name="octicon-chevron-left" :size="16" class="tw-mr-1"/>
            </a>
            <a class="active item tw-py-1">{{ page }}</a>
            <a
              class="item navigation" :class="{'disabled': page === finalPage}"
              @click="changePage(page + 1)" :title="textNextPage"
            >
              <svg-icon name="octicon-chevron-right" :size="16" class="tw-ml-1"/>
            </a>
            <a
              class="item navigation tw-py-1" :class="{'disabled': page === finalPage}"
              @click="changePage(finalPage)" :title="textLastPage"
            >
              <svg-icon name="gitea-double-chevron-right" :size="16" class="tw-ml-1"/>
            </a>
          </div>
        </div>
      </div>
    </div>
    <div v-if="!isOrganization" v-show="tab === 'organizations'" class="ui tab active list dashboard-orgs">
      <h4 class="ui top attached header tw-flex tw-items-center">
        <div class="tw-flex-1 tw-flex tw-items-center">
          {{ textMyOrgs }}
          <span class="ui grey label tw-ml-2">{{ organizationsTotalCount }}</span>
        </div>
        <a class="tw-flex tw-items-center muted" v-if="canCreateOrganization" :href="subUrl + '/org/create'" :data-tooltip-content="textNewOrg">
          <svg-icon name="octicon-plus"/>
        </a>
      </h4>
      <div v-if="!organizations.length" class="ui attached segment">
        <div class="empty-repo-or-org">
          <svg-icon name="octicon-organization" :size="24"/>
          <p>{{ textNoOrg }}</p>
        </div>
      </div>
      <div v-else class="ui attached table segment tw-rounded-b">
        <ul class="repo-owner-name-list">
          <li class="tw-flex tw-items-center tw-py-2" v-for="org in organizations" :key="org.name">
            <a class="repo-list-link muted" :href="subUrl + '/' + encodeURIComponent(org.name)">
              <svg-icon name="octicon-organization" :size="16" class="repo-list-icon"/>
              <div class="text truncate">{{ org.full_name ? `${org.full_name} (${org.name})` : org.name }}</div>
              <div><!-- div to prevent underline of label on hover -->
                <span class="ui tiny basic label" v-if="org.org_visibility !== 'public'">
                  {{ org.org_visibility === 'limited' ? textOrgVisibilityLimited: textOrgVisibilityPrivate }}
                </span>
              </div>
            </a>
            <div class="text light grey tw-flex tw-items-center tw-ml-2">
              {{ org.num_repos }}
              <svg-icon name="octicon-repo" :size="16" class="tw-ml-1 tw-mt-0.5"/>
            </div>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>
<style scoped>
ul {
  list-style: none;
  margin: 0;
  padding-left: 0;
}

ul li {
  padding: 0 10px;
}

ul li:not(:last-child) {
  border-bottom: 1px solid var(--color-secondary);
}

.repos-search {
  padding-bottom: 0 !important;
}

.repos-filter {
  margin-top: 0 !important;
  border-bottom-width: 0 !important;
}

.repos-filter .item {
  padding-left: 6px !important;
  padding-right: 6px !important;
}

.repo-list-link {
  min-width: 0; /* for text truncation */
  display: flex;
  align-items: center;
  flex: 1;
  gap: 0.5rem;
}

.repo-list-link .svg {
  color: var(--color-text-light-2);
}

.repo-list-icon {
  min-width: 16px;
  margin-right: 2px;
}

/* octicon-mirror has no padding inside the SVG */
.repo-list-icon.octicon-mirror {
  width: 14px;
  min-width: 14px;
  margin-left: 1px;
  margin-right: 3px;
}

.repo-owner-name-list li.active {
  background: var(--color-hover);
}

.empty-repo-or-org {
  margin-top: 1em;
  text-align: center;
  color: var(--color-placeholder-text);
}

.empty-repo-or-org p {
  margin: 1em auto;
}
</style>
