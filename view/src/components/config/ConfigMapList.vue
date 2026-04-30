<script setup>
import { ref } from 'vue'

const configmaps = ref([
  { name: 'kube-root-ca.crt', namespace: 'default', data: 1, age: '145d' },
  { name: 'web-app-config', namespace: 'default', data: 4, age: '14d' },
  { name: 'coredns', namespace: 'kube-system', data: 1, age: '145d' },
  { name: 'prometheus-server-conf', namespace: 'monitoring', data: 3, age: '42d' },
])
</script>

<template>
  <div class="cm-view">
    <div class="header">
      <div class="title">Config Maps</div>
      <div class="subtitle">Non-confidential data stored in key-value pairs</div>
    </div>

    <div class="cm-list">
      <div class="cm-header-row">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        <div class="col-data">Data Keys</div>
        <div class="col-age">Age</div>
      </div>

      <div v-for="cm in configmaps" :key="cm.name" class="cm-row">
        <div class="col-name">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #6ba3f9; margin-right: 8px;"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline><line x1="16" y1="13" x2="8" y2="13"></line><line x1="16" y1="17" x2="8" y2="17"></line><polyline points="10 9 9 9 8 9"></polyline></svg>
          {{ cm.name }}
        </div>
        <div class="col-ns font-mono">{{ cm.namespace }}</div>
        <div class="col-data font-mono">{{ cm.data }} keys</div>
        <div class="col-age font-mono">{{ cm.age }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.cm-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; height: 100%; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.cm-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }

.cm-header-row {
  display: grid;
  grid-template-columns: 2fr 1fr 100px 100px;
  gap: 16px;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 11px;
  font-weight: 600;
  color: #8b8f96;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.cm-row {
  display: grid;
  grid-template-columns: 2fr 1fr 100px 100px;
  gap: 16px;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
}
.cm-row:last-child { border-bottom: none; }
.cm-row:hover { background: rgba(255, 255, 255, 0.02); }

.col-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: 'SF Mono', Consolas, monospace; color: #b0b4ba; font-size: 12px; }
</style>
