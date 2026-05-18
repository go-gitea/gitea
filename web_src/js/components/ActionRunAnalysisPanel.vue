<script lang="ts" setup>
import {ref, computed, watch} from 'vue';
import {GET, PUT, DELETE} from '../modules/fetch.ts';

const props = defineProps<{
  runLink: string;
  attempt: number;
  failureTagsUrl: string;
  locale: Record<string, string>;
}>();

type Tag = {id: number; name: string; color: string};

const loading = ref(true);
const exists = ref(false);
const canEdit = ref(false);
const note = ref('');
const tags = ref<Tag[]>([]);
const editing = ref(false);

const allTags = ref<Tag[]>([]);
const draftNote = ref('');
const draftTagIDs = ref<Set<number>>(new Set());
const saving = ref(false);
const error = ref<string | null>(null);

const analysisURL = computed(() => `${props.runLink}/analysis?attempt=${props.attempt}`);

async function loadAnalysis() {
  loading.value = true;
  editing.value = false;
  try {
    const r = await GET(analysisURL.value);
    if (!r.ok) {
      exists.value = false;
      canEdit.value = false;
      return;
    }
    const data = await r.json();
    exists.value = data.exists;
    note.value = data.note ?? '';
    tags.value = data.tags ?? [];
    canEdit.value = !!data.canEdit;
  } finally {
    loading.value = false;
  }
}

async function loadAllTags() {
  try {
    const r = await GET(props.failureTagsUrl);
    if (r.ok) allTags.value = await r.json();
  } catch {
    // tags endpoint may fail for users without action read; ignore
  }
}

function startEdit() {
  draftNote.value = note.value;
  draftTagIDs.value = new Set(tags.value.map((t) => t.id));
  editing.value = true;
  if (!allTags.value.length) loadAllTags();
}

function cancelEdit() {
  editing.value = false;
  error.value = null;
}

function toggleTag(id: number) {
  if (draftTagIDs.value.has(id)) draftTagIDs.value.delete(id);
  else draftTagIDs.value.add(id);
}

async function save() {
  saving.value = true;
  error.value = null;
  try {
    const r = await PUT(analysisURL.value, {data: {note: draftNote.value, tag_ids: [...draftTagIDs.value]}});
    if (!r.ok) {
      error.value = await r.text();
      return;
    }
    const data = await r.json();
    exists.value = data.exists;
    note.value = data.note ?? '';
    tags.value = data.tags ?? [];
    editing.value = false;
  } finally {
    saving.value = false;
  }
}

async function remove() {
  if (!window.confirm(props.locale.confirmDelete)) return;
  const r = await DELETE(analysisURL.value);
  if (r.ok) {
    exists.value = false;
    note.value = '';
    tags.value = [];
  }
}

watch(() => props.attempt, loadAnalysis, {immediate: true});
</script>

<template>
  <div class="action-run-analysis-panel tw-mb-4">
    <h4 class="ui top attached header action-run-analysis-header">
      <div class="action-run-analysis-header-left">
        <strong class="action-run-analysis-title">{{ locale.title }}</strong>
        <template v-if="exists">
          <span
            v-for="tag in tags"
            :key="tag.id"
            class="ui label action-run-analysis-tag"
            :style="`background-color: ${tag.color}; color: white; border-color: ${tag.color}`"
          >{{ tag.name }}</span>
        </template>
      </div>
      <div v-if="canEdit && !editing" class="action-run-analysis-header-right">
        <button v-if="exists" class="ui mini button" @click="startEdit">{{ locale.edit }}</button>
        <button v-if="exists" class="ui mini red button" @click="remove">{{ locale.delete }}</button>
        <button v-else class="ui mini primary button" @click="startEdit">{{ locale.add }}</button>
      </div>
    </h4>
    <div class="ui attached segment">
      <div v-if="loading" class="text muted">…</div>
      <div v-else-if="editing">
        <textarea
          v-model="draftNote"
          rows="6"
          class="tw-w-full"
          :placeholder="locale.notePlaceholder"
        />
        <div class="tw-my-2">
          <strong>{{ locale.tagsLabel }}</strong>
          <div class="tw-flex tw-flex-wrap tw-gap-1 tw-mt-1">
            <button
              v-for="tag in allTags"
              :key="tag.id"
              type="button"
              class="ui label"
              :style="draftTagIDs.has(tag.id) ? `background-color: ${tag.color}; color: white; border-color: ${tag.color}` : ''"
              @click="toggleTag(tag.id)"
            >
              {{ tag.name }}
            </button>
            <span v-if="!allTags.length" class="text muted">—</span>
          </div>
        </div>
        <div v-if="error" class="ui negative message">{{ error }}</div>
        <button class="ui primary button" :disabled="saving" @click="save">{{ locale.save }}</button>
        <button class="ui button" :disabled="saving" @click="cancelEdit">{{ locale.cancel }}</button>
      </div>
      <pre v-else-if="exists" class="tw-whitespace-pre-wrap tw-break-words tw-m-0 tw-font-sans">{{ note }}</pre>
      <div v-else class="text muted">{{ locale.empty }}</div>
    </div>
  </div>
</template>

<style scoped>
.action-run-analysis-header {
  display: flex !important;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex-wrap: wrap;
}
.action-run-analysis-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  min-width: 0;
}
.action-run-analysis-title {
  font-size: 1em;
}
.action-run-analysis-tag {
  font-size: 0.85em !important;
}
.action-run-analysis-header-right {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}
</style>
