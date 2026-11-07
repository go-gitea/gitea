import {queryElems, toggleElem} from '../utils/dom.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';

function initOrgTeamSettings(pageContent: HTMLElement) {
  queryElems(pageContent, 'input[name=permission]', (el) => el.addEventListener('change', () => {
    // Change team access mode
    const val = pageContent.querySelector<HTMLInputElement>('input[name=permission]:checked')?.value;
    toggleElem(pageContent.querySelectorAll('.team-units'), val !== 'admin' && val !== 'write');
  }));
}

export function initOrgTeam() {
  registerGlobalInitFunc('initOrgTeamSettings', initOrgTeamSettings);
}
