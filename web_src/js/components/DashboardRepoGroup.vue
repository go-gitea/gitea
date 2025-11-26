<script setup lang="ts">
import type {SortableOptions} from 'sortablejs';
import DashboardRepoGroupItem from './DashboardRepoGroupItem.vue';
import {Sortable} from 'sortablejs-vue3';
import hash from 'object-hash';
import {computed, inject, nextTick, type ComputedRef, type WritableComputedRef} from 'vue';
import {GET, POST} from '../modules/fetch.ts';
import type {GroupMapType} from './DashboardRepoList.vue';
const {curGroup, depth} = defineProps<{ curGroup: number; depth: number; }>();
const emitter = defineEmits<{
  loadChanged: [ boolean ],
  itemAdded: [ item: any, index: number ],
  itemRemoved: [ item: any, index: number ]
}>();
const groupData = inject<WritableComputedRef<Map<number, GroupMapType>>>('groups');
const searchUrl = inject<string>('searchURL');
const orgName = inject<string>('orgName');

const combined = computed(() => {
  let groups = groupData.value.get(curGroup)?.subgroups ?? [];
  groups = Array.from(new Set(groups));

  const repos = (groupData.value.get(curGroup)?.repos ?? []).filter((a, pos, arr) => arr.findIndex((b) => b.id === a.id) === pos);
  const c = [
    ...groups, // ,
    ...repos,
  ];
  return c;
});
function repoMapper(webSearchRepo: any) {
  return {
    ...webSearchRepo.repository,
    latest_commit_status_state: webSearchRepo.latest_commit_status?.State, // if latest_commit_status is null, it means there is no commit status
    latest_commit_status_state_link: webSearchRepo.latest_commit_status?.TargetURL,
    locale_latest_commit_status_state: webSearchRepo.locale_latest_commit_status,
  };
}
function mapper(item: any) {
  groupData.value.set(item.group.id, {
    repos: item.repos.map((a: any) => repoMapper(a)),
    subgroups: item.subgroups.map((a: {group: any}) => a.group.id),
    ...item.group,
    latest_commit_status_state: item.latest_commit_status?.State, // if latest_commit_status is null, it means there is no commit status
    latest_commit_status_state_link: item.latest_commit_status?.TargetURL,
    locale_latest_commit_status_state: item.locale_latest_commit_status,
  });
  // return {
  // ...item.group,
  // subgroups: item.subgroups.map((a) => mapper(a)),
  // repos: item.repos.map((a) => repoMapper(a)),
  // };
}
async function searchGroup(gid: number) {
  emitter('loadChanged', true);
  const searchedURL = `${searchUrl}&group_id=${gid}`;
  let response, json;
  try {
    response = await GET(searchedURL);
    json = await response.json();
  } catch {
    emitter('loadChanged', false);
    return;
  }
  mapper(json.data);
  for (const g of json.data.subgroups) {
    mapper(g);
  }
  emitter('loadChanged', false);
  const tmp = groupData.value;
  groupData.value = tmp;
}
const orepos = inject<ComputedRef<any[]>>('repos');

const dynKey = computed(() => hash(combined.value));
function getId(it: any) {
  if (typeof it === 'number') {
    return `group-${it}`;
  }
  return `repo-${it.id}`;
}

const options: SortableOptions = {
  group: {
    name: 'repo-group',
    put(to, _from, _drag, _ev) {
      const closestLi = to.el?.closest('li');
      const base = to.el.getAttribute('data-is-group').toLowerCase() === 'true';
      if (closestLi) {
        const input = Array.from(closestLi?.querySelector('label')?.children).find((a) => a instanceof HTMLInputElement && a.checked);
        return base && Boolean(input);
      }
      return base;
    },
    pull: true,
  },
  delay: 500,
  emptyInsertThreshold: 50,
  delayOnTouchOnly: true,
  dataIdAttr: 'data-sort-id',
  draggable: '.expandable-menu-item',
  dragClass: 'active',
  store: {
    get() {
      return combined.value.map((a) => getId(a)).filter((a, i, arr) => arr.indexOf(a) === i);
    },
    async set(sortable) {
      const arr = sortable.toArray();
      const groups = Array.from(new Set(arr.filter((a) => a.startsWith('group')).map((a) => parseInt(a.split('-')[1]))));
      const repos = arr
        .filter((a) => a.startsWith('repo'))
        .map((a) => orepos.value.filter(Boolean).find((b) => b.id === parseInt(a.split('-')[1])))
        .map((a, i) => ({...a, group_sort_order: i + 1}))
        .filter((a, pos, arr) => arr.findIndex((b) => b.id === a.id) === pos);

      for (let i = 0; i < groups.length; i++) {
        const cur = groupData.value.get(groups[i]);
        groupData.value.set(groups[i], {
          ...cur,
          sort_order: i + 1,
        });
      }
      const cur = groupData.value.get(curGroup);
      const ndata: GroupMapType = {
        ...cur,
        subgroups: groups.toSorted((a, b) => groupData.value.get(a).sort_order - groupData.value.get(b).sort_order),
        repos: repos.toSorted((a, b) => a.group_sort_order - b.group_sort_order),
      };
      groupData.value.set(curGroup, ndata);
      // const tmp = groupData.value;
      // groupData.value = tmp;
      for (let i = 0; i < ndata.subgroups.length; i++) {
        const sg = ndata.subgroups[i];
        const data = {
          newParent: curGroup,
          id: sg,
          newPos: i + 1,
          isGroup: true,
        };
        try {
          await POST(`/${orgName}/groups/items/move`, {
            data,
          });
        } catch (error) {
          console.error(error);
        }
      }
      for (const r of ndata.repos) {
        const data = {
          newParent: curGroup,
          id: r.id,
          newPos: r.group_sort_order,
          isGroup: false,
        };
        try {
          await POST(`/${orgName}/groups/items/move`, {
            data,
          });
        } catch (error) {
          console.error(error);
        }
      }
      nextTick(() => {
        const finalSorted = [
          ...ndata.subgroups,
          ...ndata.repos,
        ].map(getId);
        try {
          sortable.sort(finalSorted, true);
        } catch {}
      });
    },
  },
};

</script>
<template>
  <Sortable
    :options="options" tag="ul"
    :class="{ 'expandable-menu': curGroup === 0, 'repo-owner-name-list': curGroup === 0, 'expandable-ul': true }"
    v-model:list="combined"
    :data-is-group="true"
    :item-key="(it: any) => getId(it)"
    :key="dynKey"
  >
    <template #item="{ element, index }">
      <dashboard-repo-group-item
        :index="index + 1"
        :item="element"
        :depth="depth + 1"
        :key="getId(element)"
        @load-requested="searchGroup"
      />
    </template>
  </Sortable>
</template>
