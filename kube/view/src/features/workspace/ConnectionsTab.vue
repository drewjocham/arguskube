<script setup>
// Feature entry for the workspace → connections tab. Re-exports the
// existing ConnectionPanel so the migration is purely about ownership
// (the feature folder owns the import boundary) without rewriting the
// panel. Future passes move ConnectionPanel.vue into this folder and
// update the test file's import path.
import ConnectionPanel from '../../components/workspace/ConnectionPanel.vue'
</script>

<template>
  <ConnectionPanel />
</template>
