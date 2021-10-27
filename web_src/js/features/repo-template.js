import {htmlEscape} from 'escape-goat';

const {appSubUrl} = window.config;

export function initRepoTemplateSearch() {
  const $repoTemplate = $('#repo_template');
  const checkTemplate = function () {
    const $templateUnits = $('#template_units');
    const $nonTemplate = $('#non_template');
    if ($repoTemplate.val() !== '' && $repoTemplate.val() !== '0') {
      $templateUnits.show();
      $nonTemplate.hide();
    } else {
      $templateUnits.hide();
      $nonTemplate.show();
    }
  };
  $repoTemplate.on('change', checkTemplate);
  checkTemplate();

  const changeOwner = function () {
    $('#repo_template_search')
      .dropdown({
        apiSettings: {
          url: `${appSubUrl}/api/v1/repos/search?q={query}&template=true&priority_owner_id=${$('#uid').val()}`,
          onResponse(response) {
            const filteredResponse = {success: true, results: []};
            filteredResponse.results.push({
              name: '',
              value: ''
            });
            // Parse the response from the api to work with our dropdown
            $.each(response.data, (_r, repo) => {
              filteredResponse.results.push({
                name: htmlEscape(repo.full_name),
                value: repo.id
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
