import $ from 'jquery';
import {htmlEscape} from 'escape-goat';
import {hideElem, showElem} from '../utils/dom.js';

const {appSubUrl} = window.config;

export function initRepoTemplateSearch() {
  const $repoTemplate = $('#repo_template');
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
    $('#repo_template_search')
      .dropdown({
        apiSettings: {
          url: `${appSubUrl}/repo/search?q={query}&template=true&priority_owner_id=${$('#uid').val()}`,
          onResponse(response) {
            const filteredResponse = {success: true, results: []};
            filteredResponse.results.push({
              name: '',
              value: ''
            });
            // Parse the response from the api to work with our dropdown
            $.each(response.data, (_r, repo) => {
              filteredResponse.results.push({
                name: htmlEscape(repo.repository.full_name),
                value: repo.repository.id
              });
            });
            return filteredResponse;
          },
          cache: false,
        },

        fullTextSearch: true
      });
  };
  $('#uid').on('change', changeOwner);
  changeOwner();
}
