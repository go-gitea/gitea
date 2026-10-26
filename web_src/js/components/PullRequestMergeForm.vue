<script lang="ts" setup>
import {computed, onMounted, shallowRef, watch} from 'vue';
import SvgIcon from './SvgIcon.vue';
import {toggleElem} from '../utils/dom.ts';

const {i18n} = window.config;

type MergeStyle = {
  name: string,
  allowed: boolean,
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

const mergeTitleFieldValue = shallowRef<string | undefined>('');
const mergeMessageFieldValue = shallowRef<string | undefined>('');
const deleteBranchAfterMerge = shallowRef(false);
const autoMergeWhenSucceed = shallowRef(false);

const mergeStyle = shallowRef('');
const mergeStyleDetail = shallowRef<MergeStyle>({name: '', allowed: false, textDoMerge: '', hideAutoMerge: false});

const showActionForm = shallowRef(false);

const mergeButtonStyleClass = computed(() => {
  if (mergeStyle.value === mergeStyleManuallyMerged) return 'red';
  if (mergeForm.allOverridableChecksOk) return 'primary';
  return autoMergeWhenSucceed.value ? 'primary' : 'red';
});

const mergeSelectStyleClass = computed(() => {
  if (mergeForm.emptyCommit) return '';
  if (mergeStyle.value === mergeStyleManuallyMerged) return 'red';
  if (!mergeForm.allOverridableChecksOk) return 'red';
  return 'primary';
});

const forceMerge = computed(() => {
  return mergeForm.canMergeNow && !mergeForm.allOverridableChecksOk;
});

watch(mergeStyle, (val) => {
  mergeStyleDetail.value = mergeForm.mergeStyles.find((e) => e.name === val)!;
  for (const elem of document.querySelectorAll('[data-pull-merge-style]')) {
    toggleElem(elem, elem.getAttribute('data-pull-merge-style') === val);
  }
});

onMounted(() => {
  let mergeStyle = mergeForm.mergeStyles.find((e) => e.allowed && e.name === mergeForm.defaultMergeStyle)?.name;
  if (!mergeStyle) mergeStyle = mergeForm.mergeStyles.find((e) => e.allowed)?.name;
  if (mergeStyle) switchMergeStyle(mergeStyle, !mergeForm.canMergeNow);
});

function toggleActionForm(show: boolean) {
  showActionForm.value = show;
  if (!show) return;
  deleteBranchAfterMerge.value = mergeForm.defaultDeleteBranchAfterMerge;
  mergeTitleFieldValue.value = mergeStyleDetail.value.mergeTitleFieldText;
  mergeMessageFieldValue.value = mergeStyleDetail.value.mergeMessageFieldText;
}

function switchMergeStyle(name: string, autoMerge = false) {
  mergeStyle.value = name;
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

      <div class="field" v-if="mergeStyle === mergeStyleManuallyMerged">
        <input type="text" name="merge_commit_id" :placeholder="mergeForm.textMergeCommitId">
      </div>

      <div class="flex-text-block tw-gap-3">
        <button class="ui button" :class="mergeButtonStyleClass" type="submit" name="do" :value="mergeStyle">
          {{ mergeStyleDetail.textDoMerge }}
          <template v-if="autoMergeWhenSucceed">
            {{ mergeForm.textAutoMergeButtonWhenSucceed }}
          </template>
        </button>

        <button class="ui button merge-cancel" type="button" @click="toggleActionForm(false)">
          {{ mergeForm.textCancel }}
        </button>

        <div class="ui checkbox" v-if="mergeForm.isPullBranchDeletable">
          <input name="delete_branch_after_merge" type="checkbox" v-model="deleteBranchAfterMerge" id="delete-branch-after-merge">
          <label for="delete-branch-after-merge">{{ mergeForm.textDeleteBranch }}</label>
        </div>
      </div>
    </form>

    <div v-if="!showActionForm" class="tw-flex">
      <div class="ui buttons merge-button dropdown-container" :class="mergeSelectStyleClass">
        <button type="button" class="ui button" @click="toggleActionForm(true)">
          <svg-icon name="octicon-git-merge"/>
          <span class="button-text">
            {{ mergeStyleDetail.textDoMerge }}
            <template v-if="autoMergeWhenSucceed">
              {{ mergeForm.textAutoMergeButtonWhenSucceed }}
            </template>
          </span>
        </button>
        <button type="button" class="dropdown-trigger ui icon button" :aria-label="i18n.more_items">
          <svg-icon name="octicon-triangle-down" :size="14"/>
        </button>
        <div class="dropdown-popup dropdown-popup-button">
          <template v-for="msd in mergeForm.mergeStyles" :key="msd.name">
            <div class="merge-style-row" v-if="msd.allowed && mergeForm.canMergeNow">
              <button type="button" class="dropdown-item action-text" @click="switchMergeStyle(msd.name)">
                {{ msd.textDoMerge }}
              </button>
              <button
                v-if="!msd.hideAutoMerge" type="button" class="dropdown-item flex-text-block auto-merge-small"
                :aria-label="msd.textDoMerge + ' ' + mergeForm.textAutoMergeButtonWhenSucceed"
                :data-tooltip-content="mergeForm.textAutoMergeWhenSucceed"
                @click="switchMergeStyle(msd.name, true)"
              >
                <svg-icon name="octicon-clock" :size="14"/>
              </button>
            </div>
            <button
              v-if="msd.allowed && !mergeForm.canMergeNow && !msd.hideAutoMerge"
              type="button" class="dropdown-item" @click="switchMergeStyle(msd.name, true)"
            >
              {{ msd.textDoMerge }} {{ mergeForm.textAutoMergeButtonWhenSucceed }}
            </button>
          </template>
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
.ui.checkbox label {
  cursor: pointer;
}

.merge-style-row {
  display: flex;
  align-items: stretch;
}

.action-text {
  padding: 0.8rem;
  flex: 1;
  width: auto;
  margin-right: 0;
}

.action-text:only-child {
  margin-right: 4px;
}

.auto-merge-small {
  flex: 0 0 40px;
  width: 40px;
  margin-left: 0;
  justify-content: center;
}
</style>
