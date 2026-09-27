<script lang="ts">
let chosenMergeStyle = ''; // survives the merge box refresh, which remounts the form
</script>

<script lang="ts" setup>
import {computed, nextTick, onMounted, onUnmounted, shallowRef, useTemplateRef, watch} from 'vue';
import SvgIcon from './SvgIcon.vue';
import {createTippy} from '../modules/tippy.ts';
import {toggleElem} from '../utils/dom.ts';
import type {Instance} from 'tippy.js';

type MergeStyle = {
  name: string,
  textDoMerge: string,
  textConfirmMerge?: string,
  textDescription: string,
  textAutoMerge?: string,
  textConfirmAutoMerge?: string,
  textBypassMerge?: string,
  textConfirmBypassMerge?: string,
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
  defaultMergeStyle: string,
  isPullBranchDeletable: boolean,
  isReady: boolean,
  mergeMessageFieldPlaceHolder: string,
  mergeStyles: MergeStyle[],
  pullHeadCommitID: string,
  showPullCommands: boolean,
  textBypassRules: string,
  textCancel: string,
  textCmdHint: string,
  textCmdMergeHint: string,
  textDeleteBranch: string,
  textMergeCommitId: string,
  textMergeTitle: string,
  textSelectMergeStyle: string,
};

const props = defineProps<{
  mergeFormProps: MergeForm,
}>();

const mergeStyleManuallyMerged = 'manually-merged';

const mergeForm = props.mergeFormProps;

const mergeTitleFieldValue = shallowRef<string | undefined>('');
const mergeMessageFieldValue = shallowRef<string | undefined>('');
const deleteBranchAfterMerge = shallowRef(false);
const forceMerge = shallowRef(false);

const findMergeStyle = (name: string) => mergeForm.mergeStyles.find((msd) => msd.name === name);
const mergeStyle = shallowRef((findMergeStyle(chosenMergeStyle) ?? findMergeStyle(mergeForm.defaultMergeStyle) ?? mergeForm.mergeStyles[0]).name);
const mergeStyleDetail = computed(() => findMergeStyle(mergeStyle.value)!);

const showActionForm = shallowRef(false);
const actionForm = useTemplateRef<HTMLFormElement>('actionForm');
const mergeButton = useTemplateRef<HTMLButtonElement>('mergeButton');
const menuTrigger = useTemplateRef<HTMLButtonElement>('menuTrigger');
const menuPanel = useTemplateRef<HTMLDivElement>('menuPanel');
let menuTippy: Instance | undefined;

const autoMergeWhenSucceed = computed(() => !mergeStyleDetail.value.hideAutoMerge && !forceMerge.value);

const buttonTexts = computed(() => {
  const detail = mergeStyleDetail.value;
  if (autoMergeWhenSucceed.value) return {button: detail.textAutoMerge, confirm: detail.textConfirmAutoMerge};
  if (forceMerge.value && detail.textBypassMerge) return {button: detail.textBypassMerge, confirm: detail.textConfirmBypassMerge};
  return {button: detail.textDoMerge, confirm: detail.textConfirmMerge ?? detail.textDoMerge};
});

const mergeButtonStyleClass = computed(() => mergeForm.isReady && mergeStyle.value !== mergeStyleManuallyMerged ? 'green' : '');

watch(mergeStyle, (val) => {
  chosenMergeStyle = val;
  for (const elem of document.querySelectorAll('[data-pull-merge-style]')) {
    toggleElem(elem, elem.getAttribute('data-pull-merge-style') === val);
  }
}, {immediate: true});

onMounted(() => {
  if (!menuTrigger.value) return;
  menuTippy = createTippy(menuTrigger.value, {
    content: menuPanel.value!,
    getReferenceClientRect: () => menuTrigger.value!.parentElement!.getBoundingClientRect(),
    theme: 'menu',
    arrow: false,
    maxWidth: 400,
    limitSizeToViewport: {vertical: true},
    placement: 'bottom-start',
    trigger: 'click',
    interactive: true,
    hideOnClick: true,
  });
});

onUnmounted(() => menuTippy?.destroy());

async function toggleActionForm(show: boolean) {
  showActionForm.value = show;
  if (show) {
    deleteBranchAfterMerge.value = mergeForm.defaultDeleteBranchAfterMerge;
    mergeTitleFieldValue.value = mergeStyleDetail.value.mergeTitleFieldText;
    mergeMessageFieldValue.value = mergeStyleDetail.value.mergeMessageFieldText;
  }
  await nextTick();
  (show ? actionForm.value!.querySelector<HTMLElement>('input:not([type="hidden"]), button[type="submit"]')! : mergeButton.value!).focus();
}

</script>

<template>
  <div>
    <div class="ui checkbox merge-box-bypass" v-if="mergeForm.canMergeNow && !mergeForm.allOverridableChecksOk">
      <input type="checkbox" v-model="forceMerge" id="merge-bypass-rules">
      <label for="merge-bypass-rules">{{ mergeForm.textBypassRules }}</label>
    </div>

    <form ref="actionForm" class="ui form form-fetch-action" v-if="showActionForm" :action="mergeForm.baseLink+'/merge'" method="post">
      <input type="hidden" name="head_commit_id" v-model="mergeForm.pullHeadCommitID">
      <input type="hidden" name="merge_when_checks_succeed" v-model="autoMergeWhenSucceed">
      <input type="hidden" name="force_merge" v-model="forceMerge">

      <template v-if="!mergeStyleDetail.hideMergeMessageTexts">
        <div class="field">
          <input type="text" name="merge_title_field" :aria-label="mergeForm.textMergeTitle" v-model="mergeTitleFieldValue">
        </div>
        <div class="field">
          <textarea name="merge_message_field" rows="5" :placeholder="mergeForm.mergeMessageFieldPlaceHolder" v-model="mergeMessageFieldValue"/>
        </div>
      </template>

      <div class="field" v-if="mergeStyle === mergeStyleManuallyMerged">
        <input type="text" name="merge_commit_id" :placeholder="mergeForm.textMergeCommitId" required>
      </div>

      <div class="flex-text-block tw-flex-wrap">
        <button class="ui button" :class="forceMerge ? 'red' : mergeButtonStyleClass" type="submit" name="do" :value="mergeStyle">
          {{ buttonTexts.confirm }}
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

    <div v-show="!showActionForm" class="flex-text-block tw-flex-wrap">
      <div class="ui buttons" :class="mergeButtonStyleClass">
        <button ref="mergeButton" class="ui button" type="button" @click="toggleActionForm(true)">
          {{ buttonTexts.button }}
        </button>
        <button v-if="mergeForm.mergeStyles.length > 1" ref="menuTrigger" class="ui icon button" type="button" :aria-label="mergeForm.textSelectMergeStyle">
          <svg-icon name="octicon-triangle-down" :size="14"/>
        </button>
      </div>

      <span v-if="mergeForm.showPullCommands" class="merge-box-cmd-hint">
        {{ mergeForm.textCmdMergeHint }}
        <a class="show-modal" href="" data-modal="#pull-merge-cmd-modal">{{ mergeForm.textCmdHint }}</a>
      </span>
    </div>
    <div v-if="mergeForm.mergeStyles.length > 1" ref="menuPanel" class="tippy-target merge-box-menu" @click="menuTippy!.hide()">
      <a v-for="msd in mergeForm.mergeStyles" :key="msd.name" class="item" role="menuitemradio" :aria-checked="msd.name === mergeStyle" @click="mergeStyle = msd.name">
        <svg-icon name="octicon-check"/>
        <div><strong>{{ msd.textDoMerge }}</strong><small>{{ msd.textDescription }}</small></div>
      </a>
    </div>
  </div>
</template>
