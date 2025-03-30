import $ from 'jquery';
import {htmlEscape} from 'escape-goat';
import {hideElem, querySingleVisibleElem, showElem, toggleElem} from '../utils/dom.ts';

const {appSubUrl} = window.config;

export function initRepoTemplateSearch() {
  const $repoTemplate = $('#repo_template');
  if (!$repoTemplate.length) return; // make sure the current page is "new repo" page

  const checkTemplate = function () {
    const $templateUnits = $('#template_units');
    const $nonTemplate = $('#non_template');
    if ($repoTemplate.val() !== '' && $repoTemplate.val() !== '0') {
      showElem($templateUnits);
      hideElem($nonTemplate);
    } else {
      hideElem($templateUnits);
      showElem($nonTemplate);
    }
  };
  $repoTemplate.on('change', checkTemplate);
  checkTemplate();

  const changeOwner = function () {
    const elUid = document.querySelector<HTMLInputElement>('#uid');
    const elForm = elUid.closest('form');
    const elSubmitButton = querySingleVisibleElem<HTMLInputElement>(elForm, '.ui.primary.button');
    const elCreateRepoErrorMessage = elForm.querySelector('#create-repo-error-message');
    const elOwnerItem = document.querySelector(`.ui.selection.owner.dropdown .menu > .item[data-value="${CSS.escape(elUid.value)}"]`);
    hideElem(elCreateRepoErrorMessage);
    elSubmitButton.disabled = false;
    if (elOwnerItem) {
      elCreateRepoErrorMessage.textContent = elOwnerItem.getAttribute('data-create-repo-disallowed-prompt') ?? '';
      const hasError = Boolean(elCreateRepoErrorMessage.textContent);
      toggleElem(elCreateRepoErrorMessage, hasError);
      elSubmitButton.disabled = hasError;
    }

    $('#repo_template_search')
      .dropdown({
        apiSettings: {
          url: `${appSubUrl}/repo/search?q={query}&template=true&priority_owner_id=${$('#uid').val()}`,
          onResponse(response) {
            const filteredResponse = {success: true, results: []};
            filteredResponse.results.push({
              name: '',
              value: '',
            });
            // Parse the response from the api to work with our dropdown
            $.each(response.data, (_r, repo) => {
              filteredResponse.results.push({
                name: htmlEscape(repo.repository.full_name),
                value: repo.repository.id,
              });
            });
            return filteredResponse;
          },
          cache: false,
        },

        fullTextSearch: true,
      });
  };
  $('#uid').on('change', changeOwner);
  changeOwner();
}
