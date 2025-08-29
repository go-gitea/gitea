import {stripTags} from '../utils.ts';
import {hideElem, queryElemChildren, showElem, type DOMEvent} from '../utils/dom.ts';
import {POST} from '../modules/fetch.ts';
import {showErrorToast, type Toast} from '../modules/toast.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

const {appSubUrl} = window.config;

export function initRepoTopicBar() {
  const mgrBtn = document.querySelector<HTMLButtonElement>('#manage_topic');
  if (!mgrBtn) return;

  const editDiv = document.querySelector('#topic_edit');
  const viewDiv = document.querySelector('#repo-topics');
  const topicDropdown = editDiv.querySelector('.ui.dropdown');
  let lastErrorToast: Toast;

  mgrBtn.addEventListener('click', () => {
    hideElem([viewDiv, mgrBtn]);
    showElem(editDiv);
    topicDropdown.querySelector<HTMLInputElement>('input.search').focus();
  });

  document.querySelector('#cancel_topic_edit').addEventListener('click', () => {
    lastErrorToast?.hideToast();
    hideElem(editDiv);
    showElem([viewDiv, mgrBtn]);
    mgrBtn.focus();
  });

  document.querySelector<HTMLButtonElement>('#save_topic').addEventListener('click', async (e: DOMEvent<MouseEvent, HTMLButtonElement>) => {
    lastErrorToast?.hideToast();
    const topics = editDiv.querySelector<HTMLInputElement>('input[name=topics]').value;

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
            // TODO: sort items in topicDropdown, or items in edit div will have different order to the items in view div
            // !!!! it SHOULD and MUST match the code in "home_sidebar_top.tmpl" !!!!
            const link = document.createElement('a');
            link.classList.add('repo-topic', 'ui', 'large', 'label', 'gt-ellipsis');
            link.href = `${appSubUrl}/explore/repos?q=${encodeURIComponent(topic)}&topic=1`;
            link.textContent = topic;
            viewDiv.append(link);
          }
        }
        hideElem(editDiv);
        showElem([viewDiv, mgrBtn]);
      }
    } else if (response.status === 422) {
      // how to test: input topic like " invalid topic " (with spaces), and select it from the list, then "Save"
      const responseData = await response.json();
      lastErrorToast = showErrorToast(responseData.message, {duration: 5000});
      if (responseData.invalidTopics && responseData.invalidTopics.length > 0) {
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

  fomanticQuery(topicDropdown).dropdown({
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
      onResponse(this: any, res: any) {
        const formattedResponse = {
          success: false,
          results: [] as Array<Record<string, any>>,
        };
        const query = stripTags(this.urlData.query.trim());
        let found_query = false;
        const current_topics = [];
        for (const el of queryElemChildren(topicDropdown, 'a.ui.label.visible')) {
          current_topics.push(el.getAttribute('data-value'));
        }

        if (res.topics) {
          let found = false;
          for (const {topic_name} of res.topics) {
            // skip currently added tags
            if (current_topics.includes(topic_name)) {
              continue;
            }

            if (topic_name.toLowerCase() === query.toLowerCase()) {
              found_query = true;
            }
            formattedResponse.results.push({description: topic_name, 'data-value': topic_name});
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
    onLabelCreate(value: string) {
      value = value.toLowerCase().trim();
      this.attr('data-value', value).contents().first().replaceWith(value);
      return fomanticQuery(this);
    },
    onAdd(addedValue: string, _addedText: any, $addedChoice: any) {
      addedValue = addedValue.toLowerCase().trim();
      $addedChoice[0].setAttribute('data-value', addedValue);
      $addedChoice[0].setAttribute('data-text', addedValue);
    },
  });
}
