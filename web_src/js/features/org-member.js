export function initOrgMemberInvite() {
  if (document.querySelector('.page-content.organization.invitemember') === null) {
    return;
  }

  /**
   * @type {HTMLFormElement | null}
   */
  const memberInvite = document.querySelector('form.member-invite');
  if (memberInvite === null) {
    return;
  }

  const orgLink = memberInvite.getAttribute('data-org-link');
  if (orgLink === null) {
    return;
  }

  /**
   * @type {HTMLSelectElement | null}
   */
  const teamSelect = memberInvite.querySelector('select[name=team]');
  if (teamSelect === null) {
    return;
  }

  memberInvite.action = `${orgLink}/teams/${teamSelect.value}/action/add`;
  teamSelect.addEventListener('change', () => {
    memberInvite.action = `${orgLink}/teams/${teamSelect.value}/action/add`;
  });
}
