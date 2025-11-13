import {queryElems, toggleElem} from '../utils/dom.ts';

function initOrgTeamAddMember() {
  const modal = document.querySelector('#add-member-to-team-modal');
  if (!modal) return;
  const elDropdown = modal.querySelector('.team_add_member_team_search');
  const form = elDropdown.closest('form');
  const baseUrl = form.getAttribute('data-action-base-link');
  const teamInput = form.querySelector<HTMLInputElement>('input[name=team]');
  const onChangeTeam = function() {
    form.setAttribute('action', `${baseUrl}/teams/${teamInput.value}/action/add`);
  };
  fomanticQuery(elDropdown).dropdown('setting', 'onChange', onChangeTeam);
}

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

export function initOrgTeam() {
  if (!document.querySelector('.page-content.organization')) return;
  initOrgTeamAddMember();
  initOrgTeamSettings();
}
