<script setup>
import { usePopeye } from '../../composables/useWails'
import PopeyeReport from '../center/PopeyeReport.vue'

const { report, loading, error, runScan } = usePopeye()
</script>

<template>
  <div class="config-audit-view">
    <PopeyeReport
      :report="report"
      :loading="loading"
      :error="error"
      @run-scan="runScan"
    />
  </div>
</template>

<style scoped>
.config-audit-view { display: flex; flex-direction: column; height: 100%; }
</style>
