<script setup>
// CustomPanel — manual/fallback connections. No API calls; just a
// display name and optional notes stored encrypted.
</script>

<template>
  <div class="panel" style="padding:12px">
    <h4>Manual / Custom Connection</h4>
    <p class="muted">
      For any service not in the built-in catalogue. Add a connection from the
      Connections tab by clicking "Connect" on the Manual / Custom tile.
    </p>
  </div>
</template>

<style scoped>
.muted { color: var(--muted); font-size: 0.9em; }
</style>
