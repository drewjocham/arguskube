<script setup>
// Terminal feature entry point. For now this re-exports the existing
// TerminalView component verbatim — the migration is "the feature owns
// the import boundary", not "rewrite the terminal". Future work moves
// the implementation files into this folder and updates the few legacy
// callers (RunbookCodeBlock, terminalDispatch store) to go through
// `invokeTool('terminal.open', cmd)` from the feature registry.
import TerminalView from '../../components/terminal/TerminalView.vue'

defineProps({
  visible: { type: Boolean, default: false },
})
</script>

<template>
  <TerminalView :visible="visible" />
</template>
