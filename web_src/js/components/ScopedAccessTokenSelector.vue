<script lang="ts" setup>
import {computed, onMounted, onUnmounted} from 'vue';
import {hideElem, showElem} from '../utils/dom.ts';

const props = defineProps<{
  isAdmin: boolean;
  noAccessLabel: string;
  readLabel: string;
  writeLabel: string;
  scopes: string[];
}>();

const categories = computed(() => {
  const categories = {
    'activitypub': {
      read: {
        value: 'read:activitypub',
        selected: props.scopes.includes('read:activitypub'),
      },
      write: {
        value: 'write:activitypub',
        selected: props.scopes.includes('write:activitypub'),
      },
    },
  };
  if (props.isAdmin) {
    categories['admin'] = {
      read: {
        value: 'read:admin',
        selected: props.scopes.includes('read:admin'),
      },
      write: {
        value: 'write:admin',
        selected: props.scopes.includes('write:admin'),
      },
    };
  }
  categories['issue'] = {
    read: {
      value: 'read:issue',
      selected: props.scopes.includes('read:issue'),
    },
    write: {
      value: 'write:issue',
      selected: props.scopes.includes('write:issue'),
    },
  };
  categories['misc'] = {
    read: {
      value: 'read:misc',
      selected: props.scopes.includes('read:misc'),
    },
    write: {
      value: 'write:misc',
      selected: props.scopes.includes('write:misc'),
    },
  };
  categories['notification'] = {
    read: {
      value: 'read:notification',
      selected: props.scopes.includes('read:notification'),
    },
    write: {
      value: 'write:notification',
      selected: props.scopes.includes('write:notification'),
    },
  };
  categories['organization'] = {
    read: {
      value: 'read:organization',
      selected: props.scopes.includes('read:organization'),
    },
    write: {
      value: 'write:organization',
      selected: props.scopes.includes('write:organization'),
    },
  };
  categories['package'] = {
    read: {
      value: 'read:package',
      selected: props.scopes.includes('read:package'),
    },
    write: {
      value: 'write:package',
      selected: props.scopes.includes('write:package'),
    },
  };
  categories['repository'] = {
    read: {
      value: 'read:repository',
      selected: props.scopes.includes('read:repository'),
    },
    write: {
      value: 'write:repository',
      selected: props.scopes.includes('write:repository'),
    },
  };
  categories['user'] = {
    read: {
      value: 'read:user',
      selected: props.scopes.includes('read:user'),
    },
    write: {
      value: 'write:user',
      selected: props.scopes.includes('write:user'),
    },
  };
  return categories;
});

onMounted(() => {
  document.querySelector('#scoped-access-submit').addEventListener('click', onClickSubmit);
});

onUnmounted(() => {
  document.querySelector('#scoped-access-submit').removeEventListener('click', onClickSubmit);
});

function onClickSubmit(e: Event) {
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
  <div v-for="(permissions, category) in categories" :key="category" class="field tw-pl-1 tw-pb-1 access-token-category">
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
        <option v-for="(permission, action) in permissions" :key="permission.value" :value="permission.value" :selected="permission.selected">
          {{ action === 'read' ? readLabel : writeLabel }}
        </option>
      </select>
    </div>
  </div>
</template>
