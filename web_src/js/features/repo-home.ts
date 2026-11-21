import {createElementFromAttrs, hideElem, queryElemChildren, showElem} from '../utils/dom.ts';
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
  const topicDropdown = initDropdown(editDiv.querySelector<HTMLElement>('.dropdown-container')!, {
    async loadItems(query, signal) {
      query = query.toLowerCase().trim();
      const response = await GET(`${appSubUrl}/explore/topics/search?q=${encodeURIComponent(query)}`, {signal});
      const {topics} = await response.json() as TopicSearchResponse;
      if (query && topics.every(({topic_name}) => topic_name !== query)) topics.unshift({topic_name: query});
      return topics.map(({topic_name}) => createElementFromAttrs('div', {class: 'dropdown-item', 'data-value': topic_name}, topic_name));
    },
  });
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
    const topics = topicDropdown.value;

    const data = new FormData();
    data.append('topics', topics);

    const response = await POST((e.target as HTMLElement).getAttribute('data-link')!, {data});

    if (response.ok) {
      const responseData = await response.json();
      if (responseData.status === 'ok') {
        queryElemChildren(viewDiv, '.repo-topic', (el) => el.remove());
        if (topics.length) {
          for (const topic of topics.split(',')) {
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
    }
  });
}
