<script lang="ts" setup>
import {computed, onMounted, onUnmounted, ref, watch} from 'vue';
import {SvgIcon} from '../svg.ts';
import {toggleElem} from '../utils/dom.ts';

const {csrfToken, pageData} = window.config;

const mergeForm = ref(pageData.pullRequestMergeForm);

const mergeTitleFieldValue = ref('');
const mergeMessageFieldValue = ref('');
const deleteBranchAfterMerge = ref(false);
const autoMergeWhenSucceed = ref(false);

const mergeStyle = ref('');
const mergeStyleDetail = ref({
  hideMergeMessageTexts: false,
  textDoMerge: '',
  mergeTitleFieldText: '',
  mergeMessageFieldText: '',
  hideAutoMerge: false,
});

const mergeStyleAllowedCount = ref(0);

const showMergeStyleMenu = ref(false);
const showActionForm = ref(false);

const mergeButtonStyleClass = computed(() => {
  if (mergeForm.value.allOverridableChecksOk) return 'primary';
  return autoMergeWhenSucceed.value ? 'primary' : 'red';
});

const forceMerge = computed(() => {
  return mergeForm.value.canMergeNow && !mergeForm.value.allOverridableChecksOk;
});

watch(mergeStyle, (val) => {
  mergeStyleDetail.value = mergeForm.value.mergeStyles.find((e: any) => e.name === val);
  for (const elem of document.querySelectorAll('[data-pull-merge-style]')) {
    toggleElem(elem, elem.getAttribute('data-pull-merge-style') === val);
  }
});

onMounted(() => {
  mergeStyleAllowedCount.value = mergeForm.value.mergeStyles.reduce((v: any, msd: any) => v + (msd.allowed ? 1 : 0), 0);

  let mergeStyle = mergeForm.value.mergeStyles.find((e: any) => e.allowed && e.name === mergeForm.value.defaultMergeStyle)?.name;
  if (!mergeStyle) mergeStyle = mergeForm.value.mergeStyles.find((e: any) => e.allowed)?.name;
  switchMergeStyle(mergeStyle, !mergeForm.value.canMergeNow);

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
  deleteBranchAfterMerge.value = mergeForm.value.defaultDeleteBranchAfterMerge;
  mergeTitleFieldValue.value = mergeStyleDetail.value.mergeTitleFieldText;
  mergeMessageFieldValue.value = mergeStyleDetail.value.mergeMessageFieldText;
}

function switchMergeStyle(name: string, autoMerge = false) {
  mergeStyle.value = name;
  autoMergeWhenSucceed.value = autoMerge;
}

function clearMergeMessage() {
  mergeMessageFieldValue.value = mergeForm.value.defaultMergeMessage;
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
      <input type="hidden" name="_csrf" :value="csrfToken">
      <input type="hidden" name="head_commit_id" v-model="mergeForm.pullHeadCommitID">
      <input type="hidden" name="merge_when_checks_succeed" v-model="autoMergeWhenSucceed">
      <input type="hidden" name="force_merge" v-model="forceMerge">

      <template v-if="!mergeStyleDetail.hideMergeMessageTexts">
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

      <div class="field" v-if="mergeStyle === 'manually-merged'">
        <input type="text" name="merge_commit_id" :placeholder="mergeForm.textMergeCommitId">
      </div>

      <button class="ui button" :class="mergeButtonStyleClass" type="submit" name="do" :value="mergeStyle">
        {{ mergeStyleDetail.textDoMerge }}
        <template v-if="autoMergeWhenSucceed">
          {{ mergeForm.textAutoMergeButtonWhenSucceed }}
        </template>
      </button>

      <button class="ui button merge-cancel" @click="toggleActionForm(false)">
        {{ mergeForm.textCancel }}
      </button>

      <div class="ui checkbox tw-ml-1" v-if="mergeForm.isPullBranchDeletable">
        <input name="delete_branch_after_merge" type="checkbox" v-model="deleteBranchAfterMerge" id="delete-branch-after-merge">
        <label for="delete-branch-after-merge">{{ mergeForm.textDeleteBranch }}</label>
      </div>
    </form>

    <div v-if="!showActionForm" class="tw-flex">
      <!-- the merge button -->
      <div class="ui buttons merge-button" :class="[mergeForm.emptyCommit ? '' : mergeForm.allOverridableChecksOk ? 'primary' : 'red']" @click="toggleActionForm(true)">
        <button class="ui button">
          <svg-icon name="octicon-git-merge"/>
          <span class="button-text">
            {{ mergeStyleDetail.textDoMerge }}
            <template v-if="autoMergeWhenSucceed">
              {{ mergeForm.textAutoMergeButtonWhenSucceed }}
            </template>
          </span>
        </button>
        <div class="ui dropdown icon button" @click.stop="showMergeStyleMenu = !showMergeStyleMenu">
          <svg-icon name="octicon-triangle-down" :size="14"/>
          <div class="menu" :class="{'show':showMergeStyleMenu}">
            <template v-for="msd in mergeForm.mergeStyles">
              <!-- if can merge now, show one action "merge now", and an action "auto merge when succeed" -->
              <div class="item" v-if="msd.allowed && mergeForm.canMergeNow" :key="msd.name" @click.stop="switchMergeStyle(msd.name)">
                <div class="action-text">
                  {{ msd.textDoMerge }}
                </div>
                <div v-if="!msd.hideAutoMerge" class="auto-merge-small" @click.stop="switchMergeStyle(msd.name, true)">
                  <svg-icon name="octicon-clock" :size="14"/>
                  <div class="auto-merge-tip">
                    {{ mergeForm.textAutoMergeWhenSucceed }}
                  </div>
                </div>
              </div>

              <!-- if can NOT merge now, only show one action "auto merge when succeed" -->
              <div class="item" v-if="msd.allowed && !mergeForm.canMergeNow && !msd.hideAutoMerge" :key="msd.name" @click.stop="switchMergeStyle(msd.name, true)">
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
        <input type="hidden" name="_csrf" :value="csrfToken">
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
  color: var(--color-info-text);
  background-color: var(--color-info-bg);
  border: 1px solid var(--color-info-border);
  border-left: none;
  padding-right: 1rem;
}

.auto-merge-small:hover {
  color: var(--color-info-text);
  background-color: var(--color-info-bg);
  border: 1px solid var(--color-info-border);
}

.auto-merge-small:hover .auto-merge-tip {
  display: flex;
}

</style>
