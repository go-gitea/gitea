<script setup lang="ts">
import {useEditor, EditorContent} from '@tiptap/vue-3';
import StarterKit from '@tiptap/starter-kit';
import {onMounted, onUnmounted} from 'vue';

const props = defineProps({
  content: String,
});

const editor = useEditor({
  content: props.content,
  extensions: [StarterKit],
  editable: true,
});

onMounted(() => {
  const el = document.querySelector('#rich-text-editor-submit');
  if (el) {
    el.addEventListener('click', onClickSubmit);
  }
});

onUnmounted(() => {
  const el = document.querySelector('#rich-text-editor-submit');
  if (el) {
    el.removeEventListener('click', onClickSubmit);
  }
});

function onClickSubmit() {
  console.log(editor.value.getHTML());
  console.log(editor.value.getJSON());
  console.log(editor.value.getText());
}
</script>

<template>
  <EditorContent :editor="editor"/>
</template>
