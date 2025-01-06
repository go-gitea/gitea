<script setup lang="ts">
import type {Editor} from '@tiptap/vue-3';
import '@tiptap/starter-kit';
import '@tiptap/extension-underline';
import '@tiptap/extension-link';
import '@tiptap/extension-text-align';
// TODO: add checklist
// import '@tiptap/extension-task-item';
// import '@tiptap/extension-task-list';
import {SvgIcon} from '../svg.ts';
import {ref} from 'vue';

const props = defineProps<{
  editor: Editor,
  enableLink: boolean,
  enableUnderline: boolean,
  enableCheckList: boolean,
  enableTextAlign: boolean,
}>();

const isOpen = ref(false);

function toggleLink() {
  const previousUrl = props.editor.getAttributes('link').href;
  const url = window.prompt('URL', previousUrl);
  // canceled
  if (url === null) {
    return;
  }
  // empty
  if (url === '') {
    props.editor
      .chain()
      .focus()
      .extendMarkRange('link')
      .unsetLink()
      .run();

    return;
  }
  // update link
  props.editor
    .chain()
    .focus()
    .extendMarkRange('link')
    .setLink({href: url})
    .run();
}
</script>
<template>
  <div v-if="props.editor" class="container">
    <div class="control-group">
      <div class="button-group tw-shadow-md">
        <!-- TODO: make a heading dropdown for all levels -->
        <button
          type="button"
          @click="editor.chain().focus().toggleHeading({level: 1}).run()"
          :disabled="!editor.can().chain().focus().toggleHeading({level: 1}).run()"
          :class="{'is-active': editor.isActive('heading', {level: 1})}"
        >
          <svg-icon name="octicon-heading"/>
        </button>
        <button
          type="button"
          @click="editor.chain().focus().toggleBold().run()"
          :disabled="!editor.can().chain().focus().toggleBold().run()"
          :class="{'is-active': editor.isActive('bold')}"
        >
          <svg-icon name="octicon-bold"/>
        </button>
        <button
          type="button"
          @click="editor.chain().focus().toggleItalic().run()"
          :disabled="!editor.can().chain().focus().toggleItalic().run()"
          :class="{'is-active': editor.isActive('italic')}"
        >
          <svg-icon name="octicon-italic"/>
        </button>
        <button
          v-if="props.enableUnderline"
          type="button"
          @click="editor.chain().focus().toggleUnderline().run()"
          :disabled="!editor.can().chain().focus().toggleUnderline().run()"
          :class="{'is-active': editor.isActive('underline')}"
        >
          underline
          <!-- <svg-icon name="octicon-italic"/> -->
        </button>
        <button
          type="button"
          @click="editor.chain().focus().toggleStrike().run()"
          :disabled="!editor.can().chain().focus().toggleStrike().run()"
          :class="{'is-active': editor.isActive('strike')}"
        >
          <svg-icon name="octicon-strikethrough"/>
        </button>
        <button
          v-if="props.enableLink"
          type="button"
          @click="toggleLink"
          :disabled="!editor.can().chain().focus().toggleLink({href:''}).run()"
          :class="{'is-active': editor.isActive('link')}"
        >
          <svg-icon name="octicon-link"/>
        </button>
        <button
          type="button"
          @click="editor.chain().focus().toggleBulletList().run()"
          :disabled="!editor.can().chain().focus().toggleBulletList().run()"
          :class="{'is-active': editor.isActive('bullet-list')}"
        >
          <svg-icon name="octicon-list-unordered"/>
        </button>
        <button
          type="button"
          @click="editor.chain().focus().toggleOrderedList().run()"
          :disabled="!editor.can().chain().focus().toggleOrderedList().run()"
          :class="{'is-active': editor.isActive('ordered-list')}"
        >
          <svg-icon name="octicon-list-ordered"/>
        </button>
        <button
          type="button"
          @click="editor.chain().focus().toggleBlockquote().run()"
          :disabled="!editor.can().chain().focus().toggleBlockquote().run()"
          :class="{'is-active': editor.isActive('block-quote')}"
        >
          <svg-icon name="octicon-quote"/>
        </button>
        <button
          type="button"
          @click="editor.chain().focus().setHorizontalRule().run()"
          :disabled="!editor.can().chain().focus().setHorizontalRule().run()"
          :class="{'is-active': editor.isActive('horizontal-rule')}"
        >
          <svg-icon name="octicon-horizontal-rule"/>
        </button>
        <div v-if="props.enableTextAlign" class="button-menu">
          <button
            type="button"
            tabindex="0"
            @click="isOpen = true"
            @blur="isOpen = false"
          >
            alignment
          </button>
          <div v-if="isOpen" class="dropdown-menu tw-flex tw-flex-row tw-shadow-md">
            <button
              type="button"
              @click="editor.chain().focus().setTextAlign('left').run()"
              :disabled="!editor.can().chain().focus().setTextAlign('left').run()"
              :class="{'is-active': editor.isActive({ textAlign: 'left' })}"
            >
              left
            </button>
            <button
              type="button"
              @click="editor.chain().focus().setTextAlign('center').run()"
              :disabled="!editor.can().chain().focus().setTextAlign('center').run()"
              :class="{'is-active': editor.isActive({ textAlign: 'center' })}"
            >
              center
            </button>
            <button
              type="button"
              @click="editor.chain().focus().setTextAlign('right').run()"
              :disabled="!editor.can().chain().focus().setTextAlign('right').run()"
              :class="{'is-active': editor.isActive({ textAlign: 'right' })}"
            >
              right
            </button>
            <button
              type="button"
              @click="editor.chain().focus().setTextAlign('justify').run()"
              :disabled="!editor.can().chain().focus().setTextAlign('justify').run()"
              :class="{'is-active': editor.isActive({ textAlign: 'justify' })}"
            >
              justify
            </button>
          </div>
        </div>
        <!--
        TODO: add checklist
        <button
          v-if="props.enableCheckList"
          type="button"
          @click="editor.chain().focus().toggleTaskList().run()"
          :class="{'is-active': editor.isActive('link')}"
        >
          <svg-icon name="octicon-tasklist"/>
        </button> -->
      </div>
    </div>
  </div>
</template>
<style scoped>
button {
    background: transparent;
    border-radius: 5px;
    margin-left: 1px;
    margin-right: 1px;
}
button:hover {
    background: var(--color-hover);
}
.control-group {
    background: transparent;
    padding: 6px;
}
.button-group {
    background: var(--color-box-header);
    border-radius: 25px;
    padding: 5px 10px 5px 10px;
}
.is-active {
  background: var(--color-active);
}
button .is-active:hover {
    background: var(--color-active);
}
.dropdown-menu {
  position: absolute;
  background: var(--color-body);
  /* border: 1px solid var(--color-secondary); */
  z-index: 1;
  border-radius: 5px;
  padding: 6px;
}
.button-menu {
  display: inline-block;
}
</style>
