<script setup lang="ts">
/* Simple, reusable legend; keeps layout file cleaner. */
</script>

<template>
  <div class="flex items-center justify-center gap-10 py-6">
    <div class="flex items-center gap-2 text-slate-600">
      <span class="inline-block w-4 h-4 rounded-full"
            style="background: radial-gradient(circle at 35% 30%, #FAFBFC 0%, #EEF2F7 60%, #E6EBF2 100%);
                   border:1px solid #DBE2EA; box-shadow: 0 1px 2px rgba(100,116,139,.18)"></span>
      <span class="text-lg">Article</span>
    </div>
    <div class="flex items-center gap-2 text-slate-600">
      <span class="inline-block w-4 h-4 rounded-full border-2 border-slate-300 bg-white"></span>
      <span class="text-lg">Point of contention</span>
    </div>
  </div>
</template>
