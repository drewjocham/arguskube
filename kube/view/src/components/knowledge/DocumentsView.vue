<script setup>
// Placeholder — original DocumentsView wasn't checked in on the
// buddy/bugfix/config-section-scroll-and-switch branch when the rest of
// its UI plumbing (import, sidebar nav entry, CenterPanel route) was.
// Replace this stub with the real implementation when you pick that work
// back up. Until then, the route is reachable but renders the message
// below so the bundle compiles.
</script>

<template>
  <div class="documents-placeholder">
    <div class="card">
      <div class="title">Documents — placeholder</div>
      <p class="body">
        This view is wired up but the implementation hasn't landed yet.
        See <code>view/src/components/knowledge/DocumentsView.vue</code>.
      </p>
    </div>
  </div>
</template>

<style scoped>
.documents-placeholder {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px;
}
.card {
  max-width: 480px;
  padding: 24px;
  border: 1px dashed var(--border);
  border-radius: 8px;
  background: var(--bg2);
  text-align: center;
}
.title { font-size: 14px; font-weight: 600; color: var(--text); margin-bottom: 8px; }
.body { font-size: 12.5px; color: var(--text2); line-height: 1.5; }
.body code {
  background: var(--bg3);
  padding: 1px 5px;
  border-radius: 3px;
  font-family: var(--mono);
  font-size: 11.5px;
}
</style>
