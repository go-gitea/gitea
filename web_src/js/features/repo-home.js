import $ from 'jquery';
import {stripTags} from '../utils.js';
import {hideElem, showElem} from '../utils/dom.js';
import {POST} from '../modules/fetch.js';

const {appSubUrl} = window.config;

export function initRepoTopicBar() {
  const mgrBtn = document.getElementById('manage_topic');
  if (!mgrBtn) return;
  const editDiv = document.getElementById('topic_edit');
  const viewDiv = document.getElementById('repo-topics');
  const saveBtn = document.getElementById('save_topic');
  const topicDropdown = editDiv.querySelector('.dropdown');
  const $topicDropdown = $(topicDropdown);
  const $topicForm = $(editDiv);
  const $topicDropdownSearch = $topicDropdown.find('input.search');
  const topicPrompts = {
    countPrompt: topicDropdown.getAttribute('data-text-count-prompt') ?? undefined,
    formatPrompt: topicDropdown.getAttribute('data-text-format-prompt') ?? undefined,
  };

  mgrBtn.addEventListener('click', () => {
    hideElem(viewDiv);
    showElem(editDiv);
    $topicDropdownSearch.trigger('focus');
  });

  $('#cancel_topic_edit').on('click', () => {
    hideElem(editDiv);
    showElem(viewDiv);
    mgrBtn.focus();
  });

  saveBtn.addEventListener('click', async () => {
    const topics = $('input[name=topics]').val();

    const data = new FormData();
    data.append('topics', topics);

    const response = await POST(saveBtn.getAttribute('data-link'), {data});

    if (response.ok) {
      const responseData = await response.json();
      if (responseData.status === 'ok') {
        $(viewDiv).children('.topic').remove();
        if (topics.length) {
          const topicArray = topics.split(',');
          topicArray.sort();
          for (const topic of topicArray) {
            const link = document.createElement('a');
            link.classList.add('ui', 'repo-topic', 'large', 'label', 'topic', 'tw-m-0');
            link.href = `${appSubUrl}/explore/repos?q=${encodeURIComponent(topic)}&topic=1`;
            link.textContent = topic;
            mgrBtn.parentNode.insertBefore(link, mgrBtn); // insert all new topics before manage button
          }
        }
        hideElem(editDiv);
        showElem(viewDiv);
      }
    } else if (response.status === 422) {
      const responseData = await response.json();
      if (responseData.invalidTopics.length > 0) {
        topicPrompts.formatPrompt = responseData.message;

        const {invalidTopics} = responseData;
        const $topicLabels = $topicDropdown.children('a.ui.label');
        for (const [index, value] of topics.split(',').entries()) {
          if (invalidTopics.includes(value)) {
            $topicLabels.eq(index).removeClass('green').addClass('red');
          }
        }
      } else {
        topicPrompts.countPrompt = responseData.message;
      }
    }

    // Always validate the form
    $topicForm.form('validate form');
  });

  $topicDropdown.dropdown({
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
        $topicDropdown.find('a.label.visible').each((_, el) => {
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
      $($addedChoice)[0].setAttribute('data-value', addedValue);
      $($addedChoice)[0].setAttribute('data-text', addedValue);
    },
  });

  $.fn.form.settings.rules.validateTopic = function (_values, regExp) {
    const $topics = $topicDropdown.children('a.ui.label');
    const status = !$topics.length || $topics.last()[0].getAttribute('data-value').match(regExp);
    if (!status) {
      $topics.last().removeClass('green').addClass('red');
    }
    return status && !$topicDropdown.children('a.ui.label.red').length;
  };

  $topicForm.form({
    on: 'change',
    inline: true,
    fields: {
      topics: {
        identifier: 'topics',
        rules: [
          {
            type: 'validateTopic',
            value: /^\s*[a-z0-9][-.a-z0-9]{0,35}\s*$/,
            prompt: topicPrompts.formatPrompt,
          },
          {
            type: 'maxCount[25]',
            prompt: topicPrompts.countPrompt,
          },
        ],
      },
    },
  });
}
