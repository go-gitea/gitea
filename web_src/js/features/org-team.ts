import {queryElems, toggleElem} from '../utils/dom.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

const {appSubUrl} = window.config;

function initOrgTeamSettings() {
  // on the page "page-content organization new team"
  const pageContent = document.querySelector('.page-content.organization.new.team');
  if (!pageContent) return;
  queryElems(pageContent, 'input[name=permission]', (el) => el.addEventListener('change', () => {
    // Change team access mode
    const val = pageContent.querySelector<HTMLInputElement>('input[name=permission]:checked')?.value;
    toggleElem(pageContent.querySelectorAll('.team-units'), val !== 'admin');
  }));
}

function initOrgTeamSearchRepoBox() {
  // on the page "page-content organization teams"
  const $searchRepoBox = fomanticQuery('#search-repo-box');
  $searchRepoBox.search({
    minCharacters: 2,
    apiSettings: {
      url: `${appSubUrl}/repo/search?q={query}&uid=${$searchRepoBox.data('uid')}`,
      onResponse(response: any) {
        const items = [];
        for (const item of response.data) {
          items.push({
            title: item.repository.full_name.split('/')[1],
            description: item.repository.full_name,
          });
        }
        return {results: items};
      },
    },
    searchFields: ['full_name'],
    showNoResults: false,
  });
}

export function initOrgTeam() {
  if (!document.querySelector('.page-content.organization')) return;
  initOrgTeamSettings();
  initOrgTeamSearchRepoBox();
}
