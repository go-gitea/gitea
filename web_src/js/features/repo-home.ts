import {stripTags} from '../utils.ts';
import {createElementFromAttrs, hideElem, queryElemChildren, showElem, toggleElem} from '../utils/dom.ts';
import {GET, POST} from '../modules/fetch.ts';
import {showErrorToast, type Toast} from '../modules/toast.ts';
import {initDropdown} from '../modules/dropdown.ts';

const {appSubUrl} = window.config;

type TopicSearchResponse = {topics: Array<{topic_name: string}>};

export function initRepoTopicBar() {
  const mgrBtn = document.querySelector<HTMLButtonElement>('#manage_topic');
  if (!mgrBtn) return;

  const editDiv = document.querySelector('#topic_edit')!;
  const viewDiv = document.querySelector('#repo-topics')!;
  const topicFromInput = createElementFromAttrs('div', {class: 'dropdown-item tw-hidden', 'data-static': ''});
  const topicDropdown = initDropdown(editDiv.querySelector<HTMLElement>('.dropdown-container')!, {
    async loadItems(query, signal) {
      query = stripTags(query).toLowerCase().trim();
      topicFromInput.setAttribute('data-value', query);
      topicFromInput.textContent = query;
      toggleElem(topicFromInput, Boolean(query) && !topicDropdown.values().includes(query));
      const response = await GET(`${appSubUrl}/explore/topics/search?q=${encodeURIComponent(query)}`, {signal});
      const {topics} = await response.json() as TopicSearchResponse;
      const selected = new Set(topicDropdown.values());
      const names = topics.map(({topic_name}) => topic_name).filter((name) => name !== query && !selected.has(name));
      if (topics.some(({topic_name}) => topic_name.toLowerCase() === query)) names.sort().reverse(); // the server order, unless the exact match comes first
      return names.map((name) => createElementFromAttrs('div', {class: 'dropdown-item', 'data-value': name}, name));
    },
  });
  topicDropdown.popup.append(topicFromInput);
  let lastErrorToast: Toast | null = null;

  mgrBtn.addEventListener('click', () => {
    hideElem([viewDiv, mgrBtn]);
    showElem(editDiv);
    topicDropdown.show();
  });

  document.querySelector('#cancel_topic_edit')!.addEventListener('click', () => {
    lastErrorToast?.hideToast();
    topicDropdown.hide();
    hideElem(editDiv);
    showElem([viewDiv, mgrBtn]);
    mgrBtn.focus();
  });

  document.querySelector<HTMLButtonElement>('#save_topic')!.addEventListener('click', async (e) => {
    lastErrorToast?.hideToast();
    const topics = editDiv.querySelector<HTMLInputElement>('input[name=topics]')!.value;

    const data = new FormData();
    data.append('topics', topics);

    const response = await POST((e.target as HTMLElement).getAttribute('data-link')!, {data});

    if (response.ok) {
      const responseData = await response.json();
      if (responseData.status === 'ok') {
        queryElemChildren(viewDiv, '.repo-topic', (el) => el.remove());
        if (topics.length) {
          const topicArray = topics.split(',');
          topicArray.sort();
          for (const topic of topicArray) {
            const link = document.createElement('a');
            link.classList.add('repo-topic', 'ui', 'large', 'label', 'gt-ellipsis');
            link.href = `${appSubUrl}/explore/repos?q=${encodeURIComponent(topic)}&topic=1`;
            link.textContent = topic;
            viewDiv.append(link);
          }
        }
        topicDropdown.hide();
        hideElem(editDiv);
        showElem([viewDiv, mgrBtn]);
      }
    } else if (response.status === 422) {
      const responseData = await response.json();
      lastErrorToast = showErrorToast(responseData.message, {duration: 5000});
      if (responseData.invalidTopics && responseData.invalidTopics.length > 0) {
        const {invalidTopics} = responseData;
        for (const chip of topicDropdown.trigger.querySelectorAll('.ui.label[data-value]')) {
          chip.classList.toggle('red', invalidTopics.includes(chip.getAttribute('data-value')));
        }
      }
    }
  });
}
