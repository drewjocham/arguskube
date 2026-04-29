<script setup>
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { marked } from 'marked'
import { EditorContent, useEditor, VueNodeViewRenderer } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import { common, createLowlight } from 'lowlight'
import CodeBlockComponent from './CodeBlockComponent.vue'

const activeFile = ref('getting_started.md')
const s3ConfigOpen = ref(false)

const lowlight = createLowlight(common)

// S3 connection state
const s3Config = ref({
  bucket: 'kubewatcher-docs',
  endpoint: 's3.us-east-1.amazonaws.com',
  accessKey: 'AKIA...',
  secretKey: '••••••••••••••••'
})

// Mock files tree
const files = ref([
  { id: '1', name: 'Incidents', type: 'folder', open: true, children: [
    { id: 'f1', name: '2024-03-01_db_outage.md', type: 'file' },
    { id: 'f2', name: 'post_mortems.md', type: 'file' }
  ]},
  { id: '2', name: 'Runbooks', type: 'folder', open: false, children: [
    { id: 'f3', name: 'redis_oom.md', type: 'file' }
  ]},
  { id: 'f4', name: 'getting_started.md', type: 'file' },
  { id: 'f5', name: 'architecture.md', type: 'file' }
])

// Mock file contents
const fileContents = ref({
  'getting_started.md': `# KubeWatcher Knowledge Base\n\nWelcome to your connected S3 notebook.\n\nThis works exactly like **Obsidian** or **Notion**. All files you create here are backed by the configured S3 bucket. You can use this space to store incident post-mortems, custom runbook documentation, and team notes.\n\n## Quick Start\n1. Type \`\`\` and press enter to create a beautiful code block.\n2. Create new folders or markdown files using the sidebar.\n3. Everything auto-saves!`,
  'architecture.md': `# Architecture\n\nOur SaaS runs in a hybrid model.\n\n- **Local Desktop:** Stores config locally\n- **Agent Pod:** Edge ML inference\n`,
  '2024-03-01_db_outage.md': `# DB Outage Post-Mortem\n\nThe primary replica went out of memory...`
})

const editor = useEditor({
  extensions: [
    StarterKit,
    CodeBlockLowlight.extend({
      addNodeView() {
        return VueNodeViewRenderer(CodeBlockComponent)
      },
    }).configure({ lowlight }),
  ],
  content: '',
  onUpdate: ({ editor }) => {
    // In a real app we might export to Markdown, but saving HTML is fine for Notion-like UI
    fileContents.value[activeFile.value] = editor.getHTML()
  }
})

watch(activeFile, (newFile) => {
  if (editor.value) {
    const content = fileContents.value[newFile] || ''
    // If it looks like raw markdown (starts with #), compile it first
    if (content.startsWith('#') || content.includes('```')) {
      editor.value.commands.setContent(marked.parse(content))
    } else {
      editor.value.commands.setContent(content)
    }
  }
})

onMounted(() => {
  if (editor.value) {
    const content = fileContents.value[activeFile.value] || ''
    editor.value.commands.setContent(marked.parse(content))
  }
})

onBeforeUnmount(() => {
  if (editor.value) {
    editor.value.destroy()
  }
})

function selectFile(filename) {
  if (!fileContents.value[filename]) {
    fileContents.value[filename] = '# ' + filename.replace('.md', '') + '\n\n'
  }
  activeFile.value = filename
  s3ConfigOpen.value = false
}

function openS3Config() {
  s3ConfigOpen.value = true
}

function saveS3Config() {
  s3ConfigOpen.value = false
}
</script>

<template>
  <div class="notebook-view">
    <!-- Obsidian Left Sidebar -->
    <div class="nb-sidebar">
      <div class="nb-header">
        <div class="nb-vault">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="12 2 2 7 12 12 22 7 12 2"></polygon><polyline points="2 17 12 22 22 17"></polyline><polyline points="2 12 12 17 22 12"></polyline></svg>
          {{ s3Config.bucket }}
        </div>
        <div class="nb-actions">
          <button class="nb-icon-btn"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline><line x1="12" y1="18" x2="12" y2="12"></line><line x1="9" y1="15" x2="15" y2="15"></line></svg></button>
          <button class="nb-icon-btn"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path><line x1="12" y1="11" x2="12" y2="17"></line><line x1="9" y1="14" x2="15" y2="14"></line></svg></button>
        </div>
      </div>

      <div class="nb-filetree">
        <div v-for="item in files" :key="item.id">
          
          <div v-if="item.type === 'folder'" class="nb-folder">
            <div class="nb-folder-name" @click="item.open = !item.open">
              <svg :class="{ open: item.open }" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"></polyline></svg>
              {{ item.name }}
            </div>
            <div v-if="item.open" class="nb-folder-children">
              <div v-for="child in item.children" :key="child.id" 
                   class="nb-file" 
                   :class="{ active: activeFile === child.name && !s3ConfigOpen }"
                   @click="selectFile(child.name)">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>
                {{ child.name }}
              </div>
            </div>
          </div>

          <div v-else class="nb-file" 
               :class="{ active: activeFile === item.name && !s3ConfigOpen }"
               @click="selectFile(item.name)">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>
            {{ item.name }}
          </div>

        </div>
      </div>

      <div class="nb-settings-btn" @click="openS3Config" :class="{ active: s3ConfigOpen }">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"></circle><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path></svg>
        S3 Vault Settings
      </div>
    </div>

    <!-- Main Content Area -->
    <div class="nb-main">
      
      <!-- S3 Config View -->
      <div v-if="s3ConfigOpen" class="nb-config-view">
        <div class="config-container">
          <div class="config-header">
            <h2>S3 Vault Configuration</h2>
            <p>Connect your KubeWatcher notebook to an S3 bucket for persistent markdown storage. This bucket will be used to store exported incident logs and team knowledge.</p>
          </div>
          
          <div class="config-form">
            <div class="form-group">
              <label>S3 Bucket Name</label>
              <input type="text" v-model="s3Config.bucket" class="form-input" />
            </div>
            
            <div class="form-group">
              <label>Endpoint URL (Optional for AWS)</label>
              <input type="text" v-model="s3Config.endpoint" class="form-input" />
            </div>

            <div class="form-row">
              <div class="form-group" style="flex:1;">
                <label>Access Key</label>
                <input type="text" v-model="s3Config.accessKey" class="form-input" />
              </div>
              <div class="form-group" style="flex:1;">
                <label>Secret Key</label>
                <input type="password" v-model="s3Config.secretKey" class="form-input" />
              </div>
            </div>

            <div class="form-actions">
              <button class="btn btn-test">Test Connection</button>
              <button class="btn btn-primary" @click="saveS3Config">Save & Sync</button>
            </div>
          </div>
        </div>
      </div>

      <!-- Markdown Editor View -->
      <div v-else class="nb-editor-view">
        <div class="editor-header">
          <div class="file-path">
            {{ activeFile }}
          </div>
          <div class="editor-status" style="display: flex; gap: 16px;">
            <div style="display: flex; align-items: center; gap: 6px;">
              <span class="status-dot"></span> Synced to S3
            </div>
          </div>
        </div>
        
        <editor-content :editor="editor" class="tiptap-editor-wrapper" />
      </div>
      
    </div>
  </div>
</template>

<style scoped>
.notebook-view {
  display: flex;
  height: 100%;
  background: #1e1e1e;
  color: #cccccc;
  font-family: -apple-system, BlinkMacSystemFont, 'Inter', sans-serif;
  overflow: hidden;
}

/* Sidebar */
.nb-sidebar {
  width: 250px;
  background: #252526;
  border-right: 1px solid #333;
  display: flex;
  flex-direction: column;
}
.nb-header {
  padding: 12px 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid #333;
}
.nb-vault {
  font-weight: 600;
  font-size: 13px;
  color: #e0e0e0;
  display: flex;
  align-items: center;
  gap: 8px;
}
.nb-actions {
  display: flex;
  gap: 6px;
}
.nb-icon-btn {
  background: transparent;
  border: none;
  color: #999;
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  display: flex;
  align-items: center;
}
.nb-icon-btn:hover { background: #333; color: #fff; }

.nb-filetree {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}
.nb-folder-name, .nb-file {
  padding: 6px 16px;
  font-size: 13px;
  color: #cccccc;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
}
.nb-folder-name:hover, .nb-file:hover { background: #2a2d2e; }
.nb-file.active { background: #37373d; color: #fff; }
.nb-folder-name svg { transition: transform 0.15s; }
.nb-folder-name svg.open { transform: rotate(90deg); }
.nb-folder-children { padding-left: 12px; }

.nb-settings-btn {
  padding: 12px 16px;
  font-size: 13px;
  color: #999;
  border-top: 1px solid #333;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
}
.nb-settings-btn:hover { background: #2a2d2e; color: #fff; }
.nb-settings-btn.active { background: #37373d; color: #fff; }

/* Main Content */
.nb-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: #1e1e1e; /* Obsidian editor bg */
}

/* Config View */
.nb-config-view {
  flex: 1;
  display: flex;
  justify-content: center;
  align-items: flex-start;
  padding-top: 60px;
  overflow-y: auto;
}
.config-container {
  width: 500px;
  background: #252526;
  border: 1px solid #333;
  border-radius: 8px;
  padding: 32px;
}
.config-header h2 { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 8px; }
.config-header p { font-size: 13px; color: #999; line-height: 1.5; margin-bottom: 24px; }

.config-form { display: flex; flex-direction: column; gap: 16px; }
.form-group { display: flex; flex-direction: column; gap: 6px; }
.form-row { display: flex; gap: 16px; }
.form-group label { font-size: 12px; font-weight: 500; color: #bbb; }
.form-input {
  background: #1e1e1e;
  border: 1px solid #333;
  padding: 10px 12px;
  border-radius: 4px;
  color: #fff;
  font-size: 13px;
  outline: none;
}
.form-input:focus { border-color: #0e639c; }

.form-actions { display: flex; justify-content: flex-end; gap: 12px; margin-top: 16px; }
.btn { padding: 8px 16px; border-radius: 4px; font-size: 13px; font-weight: 500; cursor: pointer; border: none; }
.btn-test { background: transparent; color: #ccc; border: 1px solid #444; }
.btn-test:hover { background: #333; }
.btn-primary { background: #0e639c; color: white; }
.btn-primary:hover { background: #1177bb; }

/* Editor View */
.nb-editor-view {
  flex: 1;
  display: flex;
  flex-direction: column;
}
.editor-header {
  padding: 12px 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.file-path { font-size: 14px; font-weight: 500; color: #999; }
.editor-status { font-size: 12px; color: #666; display: flex; align-items: center; gap: 6px; }
.status-dot { width: 6px; height: 6px; border-radius: 50%; background: #3ecf8e; }

/* TipTap Editor Styles */
.tiptap-editor-wrapper {
  flex: 1;
  overflow-y: auto;
  padding: 0 48px 48px 48px;
}

:deep(.tiptap) {
  outline: none;
  color: #cccccc;
  font-size: 15px;
  line-height: 1.6;
}

:deep(.tiptap p) { margin-bottom: 1em; }
:deep(.tiptap p.is-editor-empty:first-child::before) {
  color: #555;
  content: attr(data-placeholder);
  float: left;
  height: 0;
  pointer-events: none;
}
:deep(.tiptap h1) { color: #fff; font-size: 2em; border-bottom: 1px solid #333; padding-bottom: 0.3em; margin-top: 1.5em; margin-bottom: 0.5em; font-weight: 600; }
:deep(.tiptap h2) { color: #fff; font-size: 1.5em; margin-top: 1.5em; margin-bottom: 0.5em; font-weight: 600; }
:deep(.tiptap h3) { color: #fff; font-size: 1.17em; margin-top: 1.5em; margin-bottom: 0.5em; font-weight: 600; }
:deep(.tiptap a) { color: #3794ff; text-decoration: none; }
:deep(.tiptap a:hover) { text-decoration: underline; }
:deep(.tiptap ul), :deep(.tiptap ol) { padding-left: 2em; margin-bottom: 1em; }
:deep(.tiptap blockquote) { border-left: 4px solid #37373d; padding-left: 1em; margin-left: 0; color: #999; }
:deep(.tiptap code) { background: rgba(255,255,255,0.1); padding: 0.2em 0.4em; border-radius: 3px; font-family: 'SF Mono', Consolas, monospace; font-size: 0.9em; }
</style>
