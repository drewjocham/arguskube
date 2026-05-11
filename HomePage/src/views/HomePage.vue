<template>
  <div>
    <TabKube v-if="activeTab === 'kube'" />
    <TabApi v-if="activeTab === 'api'" />
    <TabData v-if="activeTab === 'data'" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import TabKube from '../components/TabKube.vue'
import TabApi from '../components/TabApi.vue'
import TabData from '../components/TabData.vue'

const route = useRoute()

const activeTab = computed<string>(() => {
  return (route.query.tab as string) || 'kube'
})
</script>
