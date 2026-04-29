<template>
  <node-view-wrapper class="custom-code-block">
    <div class="code-block-header">
      <div class="code-actions">
        <!-- Rendered Code Content goes here -->
        <pre><node-view-content as="code" /></pre>
      </div>
    </div>
    <div class="code-block-footer">
      <div class="language-selector">
        <select v-model="selectedLanguage" class="lang-select">
          <option value="null">Code</option>
          <option v-for="lang in languages" :key="lang" :value="lang">
            {{ lang }}
          </option>
        </select>
      </div>
      <div class="footer-actions">
        <button class="action-btn" @click="copyCode" title="Copy to clipboard">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
        </button>
      </div>
    </div>
  </node-view-wrapper>
</template>

<script setup>
import { computed } from 'vue'
import { NodeViewWrapper, NodeViewContent, nodeViewProps } from '@tiptap/vue-3'

const props = defineProps(nodeViewProps)

const languages = ['bash', 'javascript', 'json', 'yaml', 'go', 'python', 'html', 'css']

const selectedLanguage = computed({
  get() {
    return props.node.attrs.language || null
  },
  set(language) {
    props.updateAttributes({ language })
  }
})

function copyCode() {
  const code = props.node.textContent
  navigator.clipboard.writeText(code)
}
</script>

<style scoped>
.custom-code-block {
  background: #0d0d0d;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  margin: 1.5rem 0;
  overflow: hidden;
  font-family: 'SF Mono', Consolas, monospace;
}

.code-block-header {
  padding: 16px;
  overflow-x: auto;
}

.code-block-header pre {
  margin: 0;
  padding: 0;
  background: transparent;
  color: #e0e0e0;
  font-size: 13px;
  line-height: 1.6;
}
.code-block-header pre code {
  background: transparent;
  padding: 0;
}

.code-block-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 16px;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
  background: rgba(255, 255, 255, 0.02);
}

.language-selector {
  display: flex;
  align-items: center;
}

.lang-select {
  background: transparent;
  border: none;
  color: #888;
  font-size: 12px;
  font-family: 'Inter', sans-serif;
  cursor: pointer;
  outline: none;
  appearance: none;
}
.lang-select:hover {
  color: #fff;
}
.lang-select option {
  background: #1e1e1e;
  color: #fff;
}

.footer-actions {
  display: flex;
  gap: 8px;
}

.action-btn {
  background: transparent;
  border: none;
  color: #888;
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.action-btn:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.1);
}

/* Syntax highlighting base overrides */
:deep(.hljs-comment), :deep(.hljs-quote) { color: #8b8f96; }
:deep(.hljs-variable), :deep(.hljs-template-variable), :deep(.hljs-attribute), :deep(.hljs-tag), :deep(.hljs-name), :deep(.hljs-regexp), :deep(.hljs-link), :deep(.hljs-name), :deep(.hljs-selector-id), :deep(.hljs-selector-class) { color: #f2777a; }
:deep(.hljs-number), :deep(.hljs-meta), :deep(.hljs-built_in), :deep(.hljs-builtin-name), :deep(.hljs-literal), :deep(.hljs-type), :deep(.hljs-params) { color: #f99157; }
:deep(.hljs-string), :deep(.hljs-symbol), :deep(.hljs-bullet) { color: #99cc99; }
:deep(.hljs-title), :deep(.hljs-section) { color: #ffcc66; }
:deep(.hljs-keyword), :deep(.hljs-selector-tag) { color: #6699cc; }
</style>
