<script setup lang="ts">
import TerminalView from '../../components/terminal/TerminalView.vue'

defineProps<{
  visible: boolean
}>()
</script>

<template>
  <TerminalView :visible="visible" />
</template>
