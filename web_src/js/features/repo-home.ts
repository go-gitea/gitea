import {createElementFromAttrs, queryElemChildren, toggleElem} from '../utils/dom.ts';
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
      query = query.toLowerCase();
      const response = await GET(`${appSubUrl}/explore/topics/search?q=${encodeURIComponent(query)}`, {signal});
      const {topics} = await response.json() as TopicSearchResponse;
      const results = topics.filter(({topic_name}) => topic_name !== query);
      if (query) results.unshift({topic_name: query});
      return results.map(({topic_name}) => createElementFromAttrs('div', {class: 'dropdown-item', 'data-value': topic_name}, topic_name));
    },
  });
  let lastErrorToast: Toast | null = null;

  const setEditing = (editing: boolean) => {
    toggleElem(editDiv, editing);
    toggleElem(mgrBtn, !editing);
    toggleElem(viewDiv, !editing && viewDiv.children.length > 0);
  };

  mgrBtn.addEventListener('click', () => {
    setEditing(true);
    topicDropdown.show();
  });

  document.querySelector('#cancel_topic_edit')!.addEventListener('click', () => {
    lastErrorToast?.hideToast();
    topicDropdown.hide();
    setEditing(false);
    mgrBtn.focus();
  });

  document.querySelector<HTMLButtonElement>('#save_topic')!.addEventListener('click', async (e) => {
    lastErrorToast?.hideToast();
    const topics = topicDropdown.values();

    const data = new FormData();
    data.append('topics', topics.join(','));

    const response = await POST((e.target as HTMLElement).getAttribute('data-link')!, {data});

    if (response.ok) {
      const responseData = await response.json();
      if (responseData.status === 'ok') {
        queryElemChildren(viewDiv, '.repo-topic', (el) => el.remove());
        viewDiv.append(...topics.sort().map((topic) => createElementFromAttrs('a', {
          class: 'repo-topic ui large label gt-ellipsis',
          title: topic,
          href: `${appSubUrl}/explore/repos?q=${encodeURIComponent(topic)}&topic=1`,
        }, topic)));
        topicDropdown.hide();
        setEditing(false);
      }
    } else if (response.status === 422) {
      const {message, invalidTopics} = await response.json() as {message: string, invalidTopics: string[] | null};
      lastErrorToast = showErrorToast(message, {duration: 5000});
      for (const topic of invalidTopics ?? []) {
        topicDropdown.trigger.querySelector(`.ui.label[data-value="${CSS.escape(topic)}"]`)?.classList.add('red');
      }
    }
  });
}
