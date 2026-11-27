<script lang="ts" setup>
import {computed, nextTick, shallowRef, useTemplateRef, watch} from 'vue';
import SvgIcon from './SvgIcon.vue';
import {closestDropdown} from '../modules/dropdown.ts';
import {trSlots} from '../modules/i18n.ts';

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
};

type MergeForm = {
  baseLink: string,
  canBypassMerge: boolean,
  canMergeNow: boolean, // includes an authorized protection bypass
  defaultMergeStyle: string,
  isMergeStateClean: boolean,
  mergeMessageFieldPlaceHolder: string,
  mergeStyles: MergeStyle[],
  pullHeadCommitID: string,
  queue?: {style: string, button: string, confirm: string, help: string, linkText: string, link: string, branch: string},
  showAutoMerge: boolean,
  textBypassRules: string,
  textCancel: string,
  textCmdInstructions: string,
  textCmdMergeHint: string,
  textMergeBlocked: string,
  textMergeCommitId: string,
  textMergeTitle: string,
  textSelectMergeStyle: string,
};

const {mergeFormProps: mergeForm} = defineProps<{
  mergeFormProps: MergeForm,
}>();
const emit = defineEmits<{mergeStyleChange: [style: string]}>();

const mergeStyleManuallyMerged = 'manually-merged';

const forceMerge = shallowRef(false);

const mergeStyle = shallowRef(mergeForm.defaultMergeStyle);
const queue = computed(() => !forceMerge.value && mergeStyle.value !== mergeStyleManuallyMerged ? mergeForm.queue : undefined);
const mergeStyleDetail = computed(() => mergeForm.mergeStyles.find((style) => style.name === mergeStyle.value)!);
const showStyleMenu = computed(() => mergeForm.mergeStyles.length > 1 && !queue.value);

const showActionForm = shallowRef(false);
const actionForm = useTemplateRef<HTMLFormElement>('actionForm');
const mergeButton = useTemplateRef<HTMLButtonElement>('mergeButton');

const autoMergeWhenSucceed = computed(() => mergeForm.showAutoMerge && mergeStyle.value !== mergeStyleManuallyMerged && !forceMerge.value);
const actionDisabled = computed(() => !autoMergeWhenSucceed.value && !mergeForm.canMergeNow && mergeStyle.value !== mergeStyleManuallyMerged);

const buttonTexts = computed(() => {
  if (queue.value) return queue.value;
  const detail = mergeStyleDetail.value;
  if (autoMergeWhenSucceed.value) return {button: detail.textAutoMerge, confirm: detail.textConfirmAutoMerge};
  if (forceMerge.value) return {button: detail.textBypassMerge, confirm: detail.textConfirmBypassMerge};
  return {button: detail.textDoMerge, confirm: detail.textConfirmMerge ?? detail.textDoMerge};
});

const mergeButtonStyleClass = computed(() => mergeForm.isMergeStateClean && mergeStyle.value !== mergeStyleManuallyMerged ? 'green' : '');

watch(mergeStyle, (val) => {
  forceMerge.value = Boolean(mergeForm.queue) && val !== mergeStyleManuallyMerged;
  emit('mergeStyleChange', val);
});

async function toggleActionForm(show: boolean) {
  if (show) closestDropdown(mergeButton.value!)?.hide();
  showActionForm.value = show;
  await nextTick();
  (show ? actionForm.value!.querySelector<HTMLElement>('input:not([type="hidden"]), button[type="submit"]')! : mergeButton.value!).focus();
}
</script>

<template>
  <div :data-pull-merge-busy="showActionForm || forceMerge ? '' : undefined">
    <div class="ui checkbox tw-flex tw-mb-4" v-if="!showActionForm && mergeForm.canBypassMerge && mergeStyle !== mergeStyleManuallyMerged">
      <input type="checkbox" v-model="forceMerge">
      <label class="tw-text-red tw-cursor-pointer">{{ mergeForm.textBypassRules }}</label>
    </div>

    <form ref="actionForm" class="ui form form-fetch-action" v-if="showActionForm" :action="mergeForm.baseLink+'/merge'" method="post">
      <input type="hidden" name="head_commit_id" :value="mergeForm.pullHeadCommitID">
      <input type="hidden" name="merge_when_checks_succeed" :value="autoMergeWhenSucceed">
      <input type="hidden" name="force_merge" :value="forceMerge">

      <template v-if="!queue && !mergeStyleDetail.hideMergeMessageTexts">
        <div class="field">
          <input type="text" name="merge_title_field" :aria-label="mergeForm.textMergeTitle" :value="mergeStyleDetail.mergeTitleFieldText">
        </div>
        <div class="field">
          <textarea name="merge_message_field" rows="5" :placeholder="mergeForm.mergeMessageFieldPlaceHolder" :value="mergeStyleDetail.mergeMessageFieldText"/>
        </div>
      </template>

      <div class="field" v-if="mergeStyle === mergeStyleManuallyMerged">
        <input type="text" name="merge_commit_id" :placeholder="mergeForm.textMergeCommitId" required>
      </div>

      <div class="flex-text-block tw-flex-wrap">
        <button class="ui button" :class="forceMerge ? 'red' : mergeButtonStyleClass" type="submit" name="do" :value="queue ? queue.style : mergeStyle" :disabled="actionDisabled">
          {{ buttonTexts.confirm }}
        </button>

        <button class="ui button" type="button" @click="toggleActionForm(false)">
          {{ mergeForm.textCancel }}
        </button>
      </div>
    </form>

    <div v-show="!showActionForm" class="flex-text-block tw-flex-wrap">
      <div class="flex-text-inline" :data-tooltip-content="actionDisabled ? mergeForm.textMergeBlocked : undefined">
        <div :key="String(showStyleMenu)" class="ui buttons" :class="[mergeButtonStyleClass, {'dropdown-container': showStyleMenu}]">
          <button ref="mergeButton" class="ui button" type="button" :disabled="actionDisabled" @click="toggleActionForm(true)">
            {{ buttonTexts.button }}
          </button>
          <template v-if="showStyleMenu">
            <button class="dropdown-trigger ui icon button" type="button" :aria-label="mergeForm.textSelectMergeStyle">
              <svg-icon name="octicon-triangle-down" :size="14"/>
            </button>
            <div class="dropdown-popup dropdown-popup-flex merge-box-menu">
              <button v-for="msd in mergeForm.mergeStyles" :key="msd.name" type="button" class="dropdown-item" role="menuitemradio" :aria-checked="msd.name === mergeStyle" @click="mergeStyle = msd.name">
                <svg-icon name="octicon-check"/>
                <div><strong>{{ msd.textDoMerge }}</strong><small>{{ msd.textDescription }}</small></div>
              </button>
            </div>
          </template>
        </div>
      </div>
      <span v-if="queue || mergeForm.textCmdInstructions" class="merge-box-footer-text tw-text-text-light">
        <tr-slots v-if="queue" :text="queue.help">
          <template #1><a :href="queue.link">{{ queue.linkText }}</a></template>
          <template #2>{{ queue.branch }}</template>
        </tr-slots>
        <span v-else>{{ mergeForm.textCmdMergeHint }}</span> <button v-if="mergeForm.textCmdInstructions" type="button" class="interact-link show-modal" data-modal="#pull-merge-cmd-modal">{{ mergeForm.textCmdInstructions }}</button>
      </span>
    </div>
  </div>
</template>
