import $ from 'jquery';
import {stripTags} from '../utils.js';
import {hideElem, showElem} from '../utils/dom.js';

const {appSubUrl, csrfToken} = window.config;

export function initRepoTopicBar() {
  const mgrBtn = $('#manage_topic');
  if (!mgrBtn.length) return;
  const editDiv = $('#topic_edit');
  const viewDiv = $('#repo-topics');
  const saveBtn = $('#save_topic');
  const topicDropdown = $('#topic_edit .dropdown');
  const topicForm = editDiv; // the old logic, editDiv is topicForm
  const topicDropdownSearch = topicDropdown.find('input.search');
  const topicPrompts = {
    countPrompt: topicDropdown.attr('data-text-count-prompt'),
    formatPrompt: topicDropdown.attr('data-text-format-prompt'),
  };

  mgrBtn.on('click', () => {
    hideElem(viewDiv);
    showElem(editDiv);
    topicDropdownSearch.focus();
  });

  $('#cancel_topic_edit').on('click', () => {
    hideElem(editDiv);
    showElem(viewDiv);
    mgrBtn.focus();
  });

  saveBtn.on('click', () => {
    const topics = $('input[name=topics]').val();

    $.post(saveBtn.attr('data-link'), {
      _csrf: csrfToken,
      topics
    }, (_data, _textStatus, xhr) => {
      if (xhr.responseJSON.status === 'ok') {
        viewDiv.children('.topic').remove();
        if (topics.length) {
          const topicArray = topics.split(',');
          topicArray.sort();
          for (let i = 0; i < topicArray.length; i++) {
            const link = $('<a class="ui repo-topic large label topic"></a>');
            link.attr('href', `${appSubUrl}/explore/repos?q=${encodeURIComponent(topicArray[i])}&topic=1`);
            link.text(topicArray[i]);
            link.insertBefore(mgrBtn); // insert all new topics before manage button
          }
        }
        hideElem(editDiv);
        showElem(viewDiv);
      }
    }).fail((xhr) => {
      if (xhr.status === 422) {
        if (xhr.responseJSON.invalidTopics.length > 0) {
          topicPrompts.formatPrompt = xhr.responseJSON.message;

          const {invalidTopics} = xhr.responseJSON;
          const topicLabels = topicDropdown.children('a.ui.label');

          for (const [index, value] of topics.split(',').entries()) {
            for (let i = 0; i < invalidTopics.length; i++) {
              if (invalidTopics[i] === value) {
                topicLabels.eq(index).removeClass('green').addClass('red');
              }
            }
          }
        } else {
          topicPrompts.countPrompt = xhr.responseJSON.message;
        }
      }
    }).always(() => {
      topicForm.form('validate form');
    });
  });

  topicDropdown.dropdown({
    allowAdditions: true,
    forceSelection: false,
    fullTextSearch: 'exact',
    fields: {name: 'description', value: 'data-value'},
    saveRemoteData: false,
    label: {
      transition: 'horizontal flip',
      duration: 200,
      variation: false,
    },
    apiSettings: {
      url: `${appSubUrl}/explore/topics/search?q={query}`,
      throttle: 500,
      cache: false,
      onResponse(res) {
        const formattedResponse = {
          success: false,
          results: [],
        };
        const query = stripTags(this.urlData.query.trim());
        let found_query = false;
        const current_topics = [];
        topicDropdown.find('a.label.visible').each((_, el) => {
          current_topics.push(el.getAttribute('data-value'));
        });

        if (res.topics) {
          let found = false;
          for (let i = 0; i < res.topics.length; i++) {
            // skip currently added tags
            if (current_topics.includes(res.topics[i].topic_name)) {
              continue;
            }

            if (res.topics[i].topic_name.toLowerCase() === query.toLowerCase()) {
              found_query = true;
            }
            formattedResponse.results.push({description: res.topics[i].topic_name, 'data-value': res.topics[i].topic_name});
            found = true;
          }
          formattedResponse.success = found;
        }

        if (query.length > 0 && !found_query) {
          formattedResponse.success = true;
          formattedResponse.results.unshift({description: query, 'data-value': query});
        } else if (query.length > 0 && found_query) {
          formattedResponse.results.sort((a, b) => {
            if (a.description.toLowerCase() === query.toLowerCase()) return -1;
            if (b.description.toLowerCase() === query.toLowerCase()) return 1;
            if (a.description > b.description) return -1;
            if (a.description < b.description) return 1;
            return 0;
          });
        }

        return formattedResponse;
      },
    },
    onLabelCreate(value) {
      value = value.toLowerCase().trim();
      this.attr('data-value', value).contents().first().replaceWith(value);
      return $(this);
    },
    onAdd(addedValue, _addedText, $addedChoice) {
      addedValue = addedValue.toLowerCase().trim();
      $($addedChoice).attr('data-value', addedValue);
      $($addedChoice).attr('data-text', addedValue);
    }
  });

  $.fn.form.settings.rules.validateTopic = function (_values, regExp) {
    const topics = topicDropdown.children('a.ui.label');
    const status = topics.length === 0 || topics.last().attr('data-value').match(regExp);
    if (!status) {
      topics.last().removeClass('green').addClass('red');
    }
    return status && topicDropdown.children('a.ui.label.red').length === 0;
  };

  topicForm.form({
    on: 'change',
    inline: true,
    fields: {
      topics: {
        identifier: 'topics',
        rules: [
          {
            type: 'validateTopic',
            value: /^[a-z0-9][a-z0-9-]{0,35}$/,
            prompt: topicPrompts.formatPrompt
          },
          {
            type: 'maxCount[25]',
            prompt: topicPrompts.countPrompt
          }
        ]
      },
    }
  });
}
