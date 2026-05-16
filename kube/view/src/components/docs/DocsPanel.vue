<script setup>
import { useGuidesStore } from '../../stores/guides'
import DocsGuideList from './DocsGuideList.vue'
import DocsStepPlayer from './DocsStepPlayer.vue'

const guides = useGuidesStore()

const hasActiveGuide = guides.activeGuideId !== null

function onSelectGuide(sectionId) {
  guides.setActiveGuide(sectionId)
}
</script>

<template>
  <div class="docs-panel">
    <DocsGuideList
      v-if="!hasActiveGuide"
      @select-guide="onSelectGuide"
    />
    <DocsStepPlayer v-else />
  </div>
</template>

<style scoped>
.docs-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}
</style>
