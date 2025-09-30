<script setup lang="ts">
/* Simple, reusable legend; keeps layout file cleaner. */
</script>

<template>
  <div class="tw-flex tw-items-center tw-justify-center tw-gap-10 tw-py-6">
    <div class="tw-flex tw-items-center tw-gap-2 tw-text-slate-600">
      <span class="inline-block tw-w-4 tw-h-4 tw-rounded-full"
            style="background: radial-gradient(circle at 35% 30%, #FAFBFC 0%, #EEF2F7 60%, #E6EBF2 100%);
                   border:1px solid #DBE2EA; box-shadow: 0 1px 2px rgba(100,116,139,.18)"></span>
      <span class="tw-text-sm">Article</span>
    </div>
    <div class="tw-flex tw-items-center tw-gap-2 tw-text-slate-600">
      <span class="inline-block tw-w-4 tw-h-4 tw-rounded-full tw-border-2 tw-bg-white"></span>
      <span class="tw-text-sm">Point of contention</span>
    </div>
  </div>
</template>
