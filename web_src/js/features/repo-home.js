import $ from 'jquery';
import {stripTags} from '../utils.js';
import {hideElem, queryElemChildren, showElem} from '../utils/dom.js';
import {POST} from '../modules/fetch.js';
import {showErrorToast} from '../modules/toast.js';

const {appSubUrl} = window.config;

export function initRepoTopicBar() {
  const mgrBtn = document.querySelector('#manage_topic');
  if (!mgrBtn) return;

  const editDiv = document.querySelector('#topic_edit');
  const viewDiv = document.querySelector('#repo-topics');
  const topicDropdown = editDiv.querySelector('.ui.dropdown');
  let lastErrorToast;

  mgrBtn.addEventListener('click', () => {
    hideElem(viewDiv);
    showElem(editDiv);
    topicDropdown.querySelector('input.search').focus();
  });

  document.querySelector('#cancel_topic_edit').addEventListener('click', () => {
    lastErrorToast?.hideToast();
    hideElem(editDiv);
    showElem(viewDiv);
    mgrBtn.focus();
  });

  document.querySelector('#save_topic').addEventListener('click', async (e) => {
    lastErrorToast?.hideToast();
    const topics = editDiv.querySelector('input[name=topics]').value;

    const data = new FormData();
    data.append('topics', topics);

    const response = await POST(e.target.getAttribute('data-link'), {data});

    if (response.ok) {
      const responseData = await response.json();
      if (responseData.status === 'ok') {
        queryElemChildren(viewDiv, '.repo-topic', (el) => el.remove());
        if (topics.length) {
          const topicArray = topics.split(',');
          topicArray.sort();
          for (const topic of topicArray) {
            // it should match the code in repo/home.tmpl
            const link = document.createElement('a');
            link.classList.add('repo-topic', 'ui', 'large', 'label');
            link.href = `${appSubUrl}/explore/repos?q=${encodeURIComponent(topic)}&topic=1`;
            link.textContent = topic;
            mgrBtn.parentNode.insertBefore(link, mgrBtn); // insert all new topics before manage button
          }
        }
        hideElem(editDiv);
        showElem(viewDiv);
      }
    } else if (response.status === 422) {
      // how to test: input topic like " invalid topic " (with spaces), and select it from the list, then "Save"
      const responseData = await response.json();
      lastErrorToast = showErrorToast(responseData.message, {duration: 5000});
      if (responseData.invalidTopics.length > 0) {
        const {invalidTopics} = responseData;
        const topicLabels = queryElemChildren(topicDropdown, 'a.ui.label');
        for (const [index, value] of topics.split(',').entries()) {
          if (invalidTopics.includes(value)) {
            topicLabels[index].classList.remove('green');
            topicLabels[index].classList.add('red');
          }
        }
      }
    }
  });

  $(topicDropdown).dropdown({
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
        for (const el of queryElemChildren(topicDropdown, 'a.ui.label.visible')) {
          current_topics.push(el.getAttribute('data-value'));
        }

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
      $addedChoice[0].setAttribute('data-value', addedValue);
      $addedChoice[0].setAttribute('data-text', addedValue);
    },
  });
}
