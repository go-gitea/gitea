import $ from 'jquery';
import {stripTags} from '../utils.js';
import {hideElem, showElem} from '../utils/dom.js';
import {htmlEscape} from 'escape-goat';
import {svg} from '../svg.js';

const {appSubUrl, csrfToken} = window.config;

export function initRepoTopicBar() {
  const mgrBtn = $('#manage_topic');
  if (!mgrBtn.length) return;

  const saveBtn = $('#save_topic');
  const topicListDiv = $('#repo-topics');
  const topicDropdown = $('#topic_edit .dropdown');
  const topicDropdownSearch = topicDropdown.find('input.search');
  const topicForm = $('#topic_edit');
  const topicPrompts = {
    countPrompt: topicDropdown.attr('data-text-count-prompt'),
    formatPrompt: topicDropdown.attr('data-text-format-prompt'),
    removeTopic: topicDropdown.attr('data-text-remove-topic'),
  };

  function addLabelDeleteIconAria($el) {
    $el.removeAttr('aria-hidden').each(function() {
      $(this).attr({
        'aria-label': topicPrompts.removeTopic.replace('%s', $(this).parent().attr('data-value')),
        'role': 'button',
      });
    });
  }

  mgrBtn.on('click', () => {
    hideElem(topicListDiv);
    showElem(topicForm);
    addLabelDeleteIconAria(topicDropdown.find('.delete.icon'));
    topicDropdownSearch.focus();
  });

  $('#cancel_topic_edit').on('click', () => {
    hideElem(topicForm);
    showElem(topicListDiv);
    mgrBtn.focus();
  });

  saveBtn.on('click', () => {
    const topics = $('input[name=topics]').val();

    $.post(saveBtn.attr('data-link'), {
      _csrf: csrfToken,
      topics
    }, (_data, _textStatus, xhr) => {
      if (xhr.responseJSON.status === 'ok') {
        topicListDiv.children('.topic').remove();
        if (topics.length) {
          const topicArray = topics.split(',');
          for (let i = 0; i < topicArray.length; i++) {
            const link = $('<a class="ui repo-topic large label topic"></a>');
            link.attr('href', `${appSubUrl}/explore/repos?q=${encodeURIComponent(topicArray[i])}&topic=1`);
            link.text(topicArray[i]);
            link.insertBefore(mgrBtn); // insert all new topics before manage button
          }
        }
        hideElem(topicForm);
        showElem(topicListDiv);
      }
    }).fail((xhr) => {
      if (xhr.status === 422) {
        if (xhr.responseJSON.invalidTopics.length > 0) {
          topicPrompts.formatPrompt = xhr.responseJSON.message;

          const {invalidTopics} = xhr.responseJSON;
          const topicLables = topicDropdown.children('a.ui.label');

          for (const [index, value] of topics.split(',').entries()) {
            for (let i = 0; i < invalidTopics.length; i++) {
              if (invalidTopics[i] === value) {
                topicLables.eq(index).removeClass('green').addClass('red');
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
    className: {
      label: 'ui small label'
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
        topicDropdown.find('div.label.visible.topic').each((_, el) => {
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
      // `this` is the default label (jQuery object), it's a `$('<a class="ui small label">')`
      // we create a new div element to replace it, to keep the same as template (repo-topic-label), because we do not want the `<a>` tag which affects aria focus.
      const $el = $(`<div class="ui small label topic gt-cursor-default" data-value="${htmlEscape(value)}">${htmlEscape(value)}${svg('octicon-x', 16, 'delete icon gt-ml-3 gt-mt-1')}</div>`);
      addLabelDeleteIconAria($el.find('.delete.icon'));
      return $el;
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
