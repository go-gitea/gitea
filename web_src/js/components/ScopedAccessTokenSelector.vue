<script lang="ts" setup>
import {computed, onMounted, onUnmounted} from 'vue';
import {hideElem, showElem} from '../utils/dom.ts';

const props = defineProps<{
  isAdmin: boolean;
  noAccessLabel: string;
  readLabel: string;
  writeLabel: string;
}>();

const categories = computed(() => {
  const categories = [
    'activitypub',
  ];
  if (props.isAdmin) {
    categories.push('admin');
  }
  categories.push(
    'issue',
    'misc',
    'notification',
    'organization',
    'package',
    'repository',
    'user');
  return categories;
});

onMounted(() => {
  document.querySelector('#scoped-access-submit').addEventListener('click', onClickSubmit);
});

onUnmounted(() => {
  document.querySelector('#scoped-access-submit').removeEventListener('click', onClickSubmit);
});

function onClickSubmit(e) {
  e.preventDefault();

  const warningEl = document.querySelector('#scoped-access-warning');
  // check that at least one scope has been selected
  for (const el of document.querySelectorAll<HTMLInputElement>('.access-token-select')) {
    if (el.value) {
      // Hide the error if it was visible from previous attempt.
      hideElem(warningEl);
      // Submit the form.
      document.querySelector<HTMLFormElement>('#scoped-access-form').submit();
      // Don't show the warning.
      return;
    }
  }
  // no scopes selected, show validation error
  showElem(warningEl);
}
</script>

<template>
  <div v-for="category in categories" :key="category" class="field tw-pl-1 tw-pb-1 access-token-category">
    <label class="category-label" :for="'access-token-scope-' + category">
      {{ category }}
    </label>
    <div class="gitea-select">
      <select
        class="ui selection access-token-select"
        name="scope"
        :id="'access-token-scope-' + category"
      >
        <option value="">
          {{ noAccessLabel }}
        </option>
        <option :value="'read:' + category">
          {{ readLabel }}
        </option>
        <option :value="'write:' + category">
          {{ writeLabel }}
        </option>
      </select>
    </div>
  </div>
</template>
