<script lang="ts" setup>
import {ref, onMounted} from 'vue';
import {GET, POST, PUT, DELETE} from '../modules/fetch.ts';

type Tag = {id: number; name: string; color: string; description: string};
type Draft = {name: string; color: string; description: string};

const DEFAULT_COLOR = DEFAULT_COLOR;

const props = defineProps<{
  apiUrl: string;
  locale: {
    name: string;
    color: string;
    description: string;
    add: string;
    edit: string;
    save: string;
    cancel: string;
    delete: string;
    confirmDelete: string;
  };
}>();

const tags = ref<Tag[]>([]);
const newDraft = ref<Draft>({name: '', color: DEFAULT_COLOR, description: ''});
const editingId = ref<number | null>(null);
const editDraft = ref<Draft>({name: '', color: DEFAULT_COLOR, description: ''});
const error = ref<string | null>(null);
const busy = ref(false);

async function reload() {
  const r = await GET(props.apiUrl);
  if (r.ok) tags.value = await r.json();
}

async function add() {
  busy.value = true;
  error.value = null;
  try {
    const r = await POST(props.apiUrl, {data: newDraft.value});
    if (!r.ok) {
      error.value = await r.text();
      return;
    }
    newDraft.value = {name: '', color: DEFAULT_COLOR, description: ''};
    await reload();
  } finally {
    busy.value = false;
  }
}

function startEdit(tag: Tag) {
  editingId.value = tag.id;
  editDraft.value = {name: tag.name, color: tag.color || DEFAULT_COLOR, description: tag.description};
}

function cancelEdit() {
  editingId.value = null;
  error.value = null;
}

async function saveEdit(id: number) {
  busy.value = true;
  error.value = null;
  try {
    const r = await PUT(`${props.apiUrl}/${id}`, {data: editDraft.value});
    if (!r.ok) {
      error.value = await r.text();
      return;
    }
    editingId.value = null;
    await reload();
  } finally {
    busy.value = false;
  }
}

async function remove(tag: Tag) {
  if (!window.confirm(props.locale.confirmDelete)) return;
  busy.value = true;
  error.value = null;
  try {
    const r = await DELETE(`${props.apiUrl}/${tag.id}`);
    if (!r.ok) {
      error.value = await r.text();
      return;
    }
    await reload();
  } finally {
    busy.value = false;
  }
}

onMounted(reload);
</script>

<template>
  <div>
    <table class="ui table">
      <thead>
        <tr>
          <th>{{ locale.name }}</th>
          <th>{{ locale.color }}</th>
          <th>{{ locale.description }}</th>
          <th/>
        </tr>
      </thead>
      <tbody>
        <template v-for="tag in tags" :key="tag.id">
          <tr v-if="editingId !== tag.id">
            <td>
              <span class="ui label" :style="`background-color: ${tag.color}; color: white`">{{ tag.name }}</span>
            </td>
            <td><code>{{ tag.color }}</code></td>
            <td>{{ tag.description }}</td>
            <td class="tw-flex tw-gap-1">
              <button class="ui button" :disabled="busy" @click="startEdit(tag)">{{ locale.edit }}</button>
              <button class="ui red button" :disabled="busy" @click="remove(tag)">{{ locale.delete }}</button>
            </td>
          </tr>
          <tr v-else>
            <td><input v-model="editDraft.name" :placeholder="locale.name"></td>
            <td><input v-model="editDraft.color" type="color"></td>
            <td><input v-model="editDraft.description" :placeholder="locale.description"></td>
            <td class="tw-flex tw-gap-1">
              <button class="ui primary button" :disabled="busy" @click="saveEdit(tag.id)">{{ locale.save }}</button>
              <button class="ui button" :disabled="busy" @click="cancelEdit">{{ locale.cancel }}</button>
            </td>
          </tr>
        </template>
        <tr>
          <td><input v-model="newDraft.name" :placeholder="locale.name"></td>
          <td><input v-model="newDraft.color" type="color"></td>
          <td><input v-model="newDraft.description" :placeholder="locale.description"></td>
          <td>
            <button class="ui primary button" :disabled="busy || !newDraft.name" @click="add">{{ locale.add }}</button>
          </td>
        </tr>
      </tbody>
    </table>
    <div v-if="error" class="ui negative message">{{ error }}</div>
  </div>
</template>
