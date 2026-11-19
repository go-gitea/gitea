<script lang="ts" setup>
import {computed, onMounted, onUnmounted, shallowRef, watch} from 'vue';
import SvgIcon from './SvgIcon.vue';
import {toggleElem} from '../utils/dom.ts';

type MergeStyle = {
  name: string,
  allowed: boolean,
  queueAction?: 'enqueue' | 'bypass',
  textDoMerge: string,
  mergeTitleFieldText?: string,
  mergeMessageFieldText?: string,
  hideMergeMessageTexts?: boolean,
  hideAutoMerge: boolean,
};

type MergeForm = {
  allOverridableChecksOk: boolean,
  baseLink: string,
  canMergeNow: boolean,
  defaultDeleteBranchAfterMerge: boolean,
  defaultMergeMessage: string,
  defaultMergeStyle: string,
  emptyCommit: boolean,
  hasPendingPullRequestMerge: boolean,
  hasPendingPullRequestMergeTip: string,
  isPullBranchDeletable: boolean,
  mergeMessageFieldPlaceHolder: string,
  mergeStyles: MergeStyle[],
  pullHeadCommitID: string,
  textAutoMergeButtonWhenSucceed: string,
  textAutoMergeCancelSchedule: string,
  textAutoMergeWhenSucceed: string,
  textCancel: string,
  textClearMergeMessage: string,
  textClearMergeMessageHint: string,
  textDeleteBranch: string,
  textMergeCommitId: string,
};

const props = defineProps<{
  mergeFormProps: MergeForm,
}>();

const mergeStyleManuallyMerged = 'manually-merged';

const mergeForm = props.mergeFormProps;
const queueStyle = mergeForm.mergeStyles.find((style) => style.queueAction === 'enqueue');

const mergeTitleFieldValue = shallowRef<string | undefined>('');
const mergeMessageFieldValue = shallowRef<string | undefined>('');
const deleteBranchAfterMerge = shallowRef(false);
const autoMergeWhenSucceed = shallowRef(false);

const mergeStyle = shallowRef<MergeStyle>({name: '', allowed: false, textDoMerge: '', hideAutoMerge: false});

const showMergeStyleMenu = shallowRef(false);
const showActionForm = shallowRef(false);

const mergeButtonStyleClass = computed(() => {
  if (mergeStyle.value.queueAction === 'bypass' || mergeStyle.value.name === mergeStyleManuallyMerged) return 'red';
  return mergeForm.allOverridableChecksOk || autoMergeWhenSucceed.value ? 'primary' : 'red';
});

const mergeSelectStyleClass = computed(() => {
  if (mergeForm.emptyCommit && mergeStyle.value.queueAction !== 'bypass') return '';
  if (mergeStyle.value.queueAction !== 'enqueue' && !mergeForm.allOverridableChecksOk) return 'red';
  return mergeButtonStyleClass.value;
});

const forceMerge = computed(() => {
  if (queueStyle) return mergeStyle.value.queueAction === 'bypass';
  return mergeForm.canMergeNow && !mergeForm.allOverridableChecksOk;
});

watch(mergeStyle, ({name}) => {
  for (const elem of document.querySelectorAll('[data-pull-merge-style]')) {
    toggleElem(elem, elem.getAttribute('data-pull-merge-style') === name);
  }
});

onMounted(() => {
  const defaultStyle = queueStyle ||
    mergeForm.mergeStyles.find((style) => style.allowed && style.name === mergeForm.defaultMergeStyle) ||
    mergeForm.mergeStyles.find((style) => style.allowed);
  if (defaultStyle) switchMergeStyle(defaultStyle, !canMergeNow(defaultStyle));

  document.addEventListener('mouseup', hideMergeStyleMenu);
});

onUnmounted(() => {
  document.removeEventListener('mouseup', hideMergeStyleMenu);
});

function hideMergeStyleMenu() {
  showMergeStyleMenu.value = false;
}

function toggleActionForm(show: boolean) {
  showActionForm.value = show;
  if (!show) return;
  deleteBranchAfterMerge.value = mergeForm.defaultDeleteBranchAfterMerge;
  mergeTitleFieldValue.value = mergeStyle.value.mergeTitleFieldText;
  mergeMessageFieldValue.value = mergeStyle.value.mergeMessageFieldText;
}

function canMergeNow(style: MergeStyle) {
  return mergeForm.canMergeNow && (style.queueAction !== 'enqueue' || mergeForm.allOverridableChecksOk);
}

function switchMergeStyle(style: MergeStyle, autoMerge = false) {
  mergeStyle.value = style;
  autoMergeWhenSucceed.value = autoMerge;
}

function clearMergeMessage() {
  mergeMessageFieldValue.value = mergeForm.defaultMergeMessage;
}
</script>

<template>
  <!--
  if this component is shown, either the user is an admin (can do a merge without checks), or they are a writer who has the permission to do a merge
  if the user is a writer and can't do a merge now (canMergeNow==false), then only show the Auto Merge for them
  How to test the UI manually:
  * Method 1: manually set some variables in pull.tmpl, eg: {{$notAllOverridableChecksOk = true}} {{$canMergeNow = false}}
  * Method 2: make a protected branch, then set state=pending/success :
    curl -X POST ${root_url}/api/v1/repos/${owner}/${repo}/statuses/${sha} \
      -H "accept: application/json" -H "authorization: Basic $base64_auth" -H "Content-Type: application/json" \
      -d '{"context": "test/context", "description": "description", "state": "${state}", "target_url": "http://localhost"}'
  -->
  <div>
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div v-if="mergeForm.hasPendingPullRequestMerge" v-html="mergeForm.hasPendingPullRequestMergeTip" class="ui info message"/>

    <!-- another similar form is in pull.tmpl (manual merge)-->
    <form class="ui form form-fetch-action" v-if="showActionForm" :action="mergeForm.baseLink+'/merge'" method="post">
      <input type="hidden" name="head_commit_id" v-model="mergeForm.pullHeadCommitID">
      <input type="hidden" name="merge_when_checks_succeed" v-model="autoMergeWhenSucceed">
      <input type="hidden" name="force_merge" :value="forceMerge">

      <template v-if="!mergeStyle.hideMergeMessageTexts">
        <div class="field">
          <input type="text" name="merge_title_field" v-model="mergeTitleFieldValue">
        </div>
        <div class="field">
          <textarea name="merge_message_field" rows="5" :placeholder="mergeForm.mergeMessageFieldPlaceHolder" v-model="mergeMessageFieldValue"/>
          <template v-if="mergeMessageFieldValue !== mergeForm.defaultMergeMessage">
            <button @click.prevent="clearMergeMessage" class="btn tw-mt-1 tw-p-1 interact-fg" :data-tooltip-content="mergeForm.textClearMergeMessageHint">
              {{ mergeForm.textClearMergeMessage }}
            </button>
          </template>
        </div>
      </template>

      <div class="field" v-if="mergeStyle.name === mergeStyleManuallyMerged">
        <input type="text" name="merge_commit_id" :placeholder="mergeForm.textMergeCommitId">
      </div>

      <div class="flex-text-block tw-gap-3">
        <button class="ui button" :class="mergeButtonStyleClass" type="submit" name="do" :value="mergeStyle.name">
          {{ mergeStyle.textDoMerge }}
          <template v-if="autoMergeWhenSucceed">
            {{ mergeForm.textAutoMergeButtonWhenSucceed }}
          </template>
        </button>

        <button class="ui button merge-cancel" type="button" @click="toggleActionForm(false)">
          {{ mergeForm.textCancel }}
        </button>

        <div class="ui checkbox" v-if="mergeForm.isPullBranchDeletable && (!queueStyle || mergeStyle.queueAction === 'bypass')">
          <input name="delete_branch_after_merge" type="checkbox" v-model="deleteBranchAfterMerge" id="delete-branch-after-merge">
          <label for="delete-branch-after-merge">{{ mergeForm.textDeleteBranch }}</label>
        </div>
      </div>
    </form>

    <div v-if="!showActionForm" class="tw-flex">
      <!-- the merge button -->
      <div class="ui buttons merge-button" :class="mergeSelectStyleClass" @click="toggleActionForm(true)">
        <button class="ui button">
          <svg-icon name="octicon-git-merge"/>
          <span class="button-text">
            {{ mergeStyle.textDoMerge }}
            <template v-if="autoMergeWhenSucceed">
              {{ mergeForm.textAutoMergeButtonWhenSucceed }}
            </template>
          </span>
        </button>
        <div class="ui dropdown icon button" @click.stop="showMergeStyleMenu = !showMergeStyleMenu">
          <svg-icon name="octicon-triangle-down" :size="14"/>
          <div class="menu" :class="{'show':showMergeStyleMenu}">
            <template v-for="msd in mergeForm.mergeStyles" :key="msd.queueAction === 'enqueue' ? 'queue' : msd.name">
              <!-- if can merge now, show one action "merge now", and an action "auto merge when succeed" -->
              <div class="item" v-if="msd.allowed && canMergeNow(msd)" @click.stop="switchMergeStyle(msd)">
                <div class="action-text" :class="{'tw-text-red': msd.queueAction === 'bypass'}">
                  {{ msd.textDoMerge }}
                </div>
                <div v-if="!msd.hideAutoMerge" class="auto-merge-small" @click.stop="switchMergeStyle(msd, true)">
                  <svg-icon name="octicon-clock" :size="14"/>
                  <div class="auto-merge-tip">
                    {{ mergeForm.textAutoMergeWhenSucceed }}
                  </div>
                </div>
              </div>

              <!-- if can NOT merge now, only show one action "auto merge when succeed" -->
              <div class="item" v-if="msd.allowed && !canMergeNow(msd) && !msd.hideAutoMerge" @click.stop="switchMergeStyle(msd, true)">
                <div class="action-text">
                  {{ msd.textDoMerge }} {{ mergeForm.textAutoMergeButtonWhenSucceed }}
                </div>
              </div>
            </template>
          </div>
        </div>
      </div>

      <!-- the cancel auto merge button -->
      <form v-if="mergeForm.hasPendingPullRequestMerge" :action="mergeForm.baseLink+'/cancel_auto_merge'" method="post" class="tw-ml-4">
        <button class="ui button">
          {{ mergeForm.textAutoMergeCancelSchedule }}
        </button>
      </form>
    </div>
  </div>
</template>

<style scoped>
/* to keep UI the same, at the moment we are still using some Fomantic UI styles, but we do not use their scripts, so we need to fine tune some styles */
.ui.dropdown .menu.show {
  display: block;
}
.ui.checkbox label {
  cursor: pointer;
}

/* make the dropdown list left-aligned */
.ui.merge-button {
  position: relative;
}
.ui.merge-button .ui.dropdown {
  position: static;
}
.ui.merge-button > .ui.dropdown:last-child > .menu:not(.left) {
  left: 0;
  right: auto;
}
.ui.merge-button .ui.dropdown .menu > .item {
  display: flex;
  align-items: stretch;
  padding: 0 !important; /* polluted by semantic.css: .ui.dropdown .menu > .item { !important } */
}

/* merge style list item */
.action-text {
  padding: 0.8rem;
  flex: 1
}

.auto-merge-small {
  width: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
}
.auto-merge-small .auto-merge-tip {
  display: none;
  left: 38px;
  top: -1px;
  bottom: -1px;
  position: absolute;
  align-items: center;
  color: var(--color-text);
  background-color: var(--color-info-bg);
  border: 1px solid var(--color-info-border);
  border-left: none;
  padding-right: 1rem;
}

.auto-merge-small:hover {
  color: var(--color-text);
  background-color: var(--color-info-bg);
  border: 1px solid var(--color-info-border);
}

.auto-merge-small:hover .auto-merge-tip {
  display: flex;
}

</style>
