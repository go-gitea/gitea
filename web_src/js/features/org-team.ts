import {queryElems, toggleElem} from '../utils/dom.ts';

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
  initOrgTeamSettings();
}
