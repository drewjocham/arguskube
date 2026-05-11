<script setup>
import { useArgusScan } from '../../composables/useWails'
import ArgusScanReport from '../center/ArgusScanReport.vue'

const { report, loading, error, runScan } = useArgusScan()
</script>

<template>
  <div class="config-audit-view">
    <ArgusScanReport
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
