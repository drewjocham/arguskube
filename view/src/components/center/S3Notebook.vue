<script setup>
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { marked } from 'marked'
import { EditorContent, useEditor, VueNodeViewRenderer } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import { common, createLowlight } from 'lowlight'
import CodeBlockComponent from './CodeBlockComponent.vue'
import { useNotebooks } from '../../composables/useWails'

const { files, loading, saving, synced, error, listFiles, getFile, saveFile, deleteFile, createFolder, testConnection, addFileToTree, moveFile } = useNotebooks()

const activeFile = ref(null)
const activeFilePath = ref(null)
const s3ConfigOpen = ref(false)
const testResult = ref(null)
const testLoading = ref(false)

const lowlight = createLowlight(common)

// S3 connection state (display-only — actual config is in Go config.go)
const s3Config = ref({
  bucket: '',
  endpoint: '',
  accessKey: '',
  secretKey: ''
})

// Track open/closed state for folders in the tree
const folderState = ref({})

// Auto-save debounce timer
let saveTimer = null

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
    if (!activeFilePath.value) return
    // Debounce auto-save: wait 800ms after last keystroke
    if (saveTimer) clearTimeout(saveTimer)
    synced.value = false
    saveTimer = setTimeout(() => {
      saveFile(activeFilePath.value, editor.getHTML())
    }, 800)
  }
})

// Load file content when selection changes
watch(activeFilePath, async (newPath) => {
  if (!newPath || !editor.value) return
  try {
    const content = await getFile(newPath)
    if (content) {
      // If raw markdown, parse it to HTML for TipTap
      if (content.startsWith('#') || content.startsWith('- ') || content.includes('```')) {
        editor.value.commands.setContent(marked.parse(content))
      } else {
        editor.value.commands.setContent(content)
      }
    } else {
      editor.value.commands.setContent('')
    }
  } catch (e) {
    // New file — initialize with heading
    const title = newPath.split('/').pop().replace('.md', '')
    const initial = `# ${title}\n\n`
    editor.value.commands.setContent(marked.parse(initial))
    saveFile(newPath, initial)
  }
})

onMounted(async () => {
  await listFiles()
  // Auto-select first file if available
  const first = findFirstFile(files.value)
  if (first) {
    selectFile(first.name, first.path)
  }
})

onBeforeUnmount(() => {
  if (editor.value) editor.value.destroy()
  if (saveTimer) clearTimeout(saveTimer)
})

function findFirstFile(entries) {
  for (const entry of entries) {
    if (entry.type === 'file') return entry
    if (entry.children?.length) {
      const found = findFirstFile(entry.children)
      if (found) return found
    }
  }
  return null
}

function selectFile(name, path) {
  activeFile.value = name
  activeFilePath.value = path
  s3ConfigOpen.value = false
}

function toggleFolder(id) {
  folderState.value[id] = !folderState.value[id]
}

function isFolderOpen(id) {
  // Default to open
  return folderState.value[id] !== false
}

function openS3Config() {
  s3ConfigOpen.value = true
}

function saveS3Config() {
  s3ConfigOpen.value = false
}

async function handleTestConnection() {
  testLoading.value = true
  testResult.value = null
  const result = await testConnection()
  testResult.value = result
  testLoading.value = false
}

async function handleCreateFile() {
  const name = prompt('File name (e.g. my-notes.md):')
  if (!name) return
  const path = name.endsWith('.md') ? name : name + '.md'
  const displayName = path.split('/').pop()
  const initial = `# ${displayName.replace('.md', '')}\n\n`
  await saveFile(path, initial)
  addFileToTree(path, displayName)
  selectFile(displayName, path)
}

async function handleCreateFolder() {
  const name = prompt('Folder name:')
  if (!name) return
  await createFolder(name)
}

async function handleDeleteFile() {
  if (!activeFilePath.value) return
  if (!confirm(`Delete ${activeFile.value}?`)) return
  await deleteFile(activeFilePath.value)
  activeFile.value = null
  activeFilePath.value = null
  if (editor.value) editor.value.commands.setContent('')
}

// --- Drag-and-drop ---
const dragOverFolder = ref(null)

function onDragStart(e, item) {
  e.dataTransfer.setData('text/plain', item.path)
  e.dataTransfer.effectAllowed = 'move'
}

function onDragOverFolder(e, folder) {
  e.preventDefault()
  e.dataTransfer.dropEffect = 'move'
  dragOverFolder.value = folder.id
}

function onDragLeaveFolder() {
  dragOverFolder.value = null
}

async function onDropOnFolder(e, folder) {
  e.preventDefault()
  dragOverFolder.value = null
  const sourcePath = e.dataTransfer.getData('text/plain')
  if (!sourcePath) return
  // Don't drop onto itself or into the folder it's already in.
  const fileName = sourcePath.split('/').pop()
  const newPath = folder.path + '/' + fileName
  if (sourcePath === newPath) return
  await moveFile(sourcePath, newPath)
  // If active file was moved, update the active path.
  if (activeFilePath.value === sourcePath) {
    activeFilePath.value = newPath
    activeFile.value = fileName
  }
  // Refresh tree.
  await listFiles()
}

async function onDropOnRoot(e) {
  e.preventDefault()
  dragOverFolder.value = null
  const sourcePath = e.dataTransfer.getData('text/plain')
  if (!sourcePath) return
  const fileName = sourcePath.split('/').pop()
  // Already at root level.
  if (!sourcePath.includes('/')) return
  const newPath = fileName
  await moveFile(sourcePath, newPath)
  if (activeFilePath.value === sourcePath) {
    activeFilePath.value = newPath
    activeFile.value = fileName
  }
  await listFiles()
}
</script>

<template>
  <div class="notebook-view">
    <!-- Obsidian Left Sidebar -->
    <div class="nb-sidebar">
      <div class="nb-header">
        <div class="nb-vault">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="12 2 2 7 12 12 22 7 12 2"></polygon><polyline points="2 17 12 22 22 17"></polyline><polyline points="2 12 12 17 22 12"></polyline></svg>
          Notebooks
        </div>
        <div class="nb-actions">
          <button class="nb-icon-btn" title="New file" @click="handleCreateFile">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline><line x1="12" y1="18" x2="12" y2="12"></line><line x1="9" y1="15" x2="15" y2="15"></line></svg>
          </button>
          <button class="nb-icon-btn" title="New folder" @click="handleCreateFolder">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path><line x1="12" y1="11" x2="12" y2="17"></line><line x1="9" y1="14" x2="15" y2="14"></line></svg>
          </button>
        </div>
      </div>

      <!-- Loading state -->
      <div v-if="loading" class="nb-loading">Loading notebooks...</div>

      <!-- Empty state -->
      <div v-else-if="files.length === 0" class="nb-empty">
        <p>No notebooks yet.</p>
        <p>Create a file or configure S3.</p>
      </div>

      <!-- File tree -->
      <div v-else class="nb-filetree"
           @dragover.prevent
           @drop="onDropOnRoot">
        <template v-for="item in files" :key="item.id">

          <div v-if="item.type === 'folder'" class="nb-folder"
               :class="{ 'drag-over': dragOverFolder === item.id }"
               @dragover="onDragOverFolder($event, item)"
               @dragleave="onDragLeaveFolder"
               @drop.stop="onDropOnFolder($event, item)">
            <div class="nb-folder-name" @click="toggleFolder(item.id)">
              <svg :class="{ open: isFolderOpen(item.id) }" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"></polyline></svg>
              {{ item.name }}
            </div>
            <div v-if="isFolderOpen(item.id) && item.children" class="nb-folder-children">
              <div v-for="child in item.children" :key="child.id"
                   class="nb-file"
                   :class="{ active: activeFilePath === child.path && !s3ConfigOpen }"
                   draggable="true"
                   @dragstart="onDragStart($event, child)"
                   @click="selectFile(child.name, child.path)">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>
                {{ child.name }}
              </div>
            </div>
          </div>

          <div v-else class="nb-file"
               :class="{ active: activeFilePath === item.path && !s3ConfigOpen }"
               draggable="true"
               @dragstart="onDragStart($event, item)"
               @click="selectFile(item.name, item.path)">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>
            {{ item.name }}
          </div>

        </template>
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
            <p>Connect your KubeWatcher notebook to an S3 bucket for persistent markdown storage. Configure S3 credentials in your environment variables (S3_BUCKET, S3_REGION, S3_ACCESS_KEY, S3_SECRET_KEY).</p>
          </div>

          <div class="config-form">
            <div class="form-group">
              <label>S3 Bucket Name</label>
              <input type="text" v-model="s3Config.bucket" class="form-input" placeholder="my-kubewatcher-docs" />
            </div>

            <div class="form-group">
              <label>Endpoint URL (Optional for AWS)</label>
              <input type="text" v-model="s3Config.endpoint" class="form-input" placeholder="s3.us-east-1.amazonaws.com" />
            </div>

            <div class="form-row">
              <div class="form-group" style="flex:1;">
                <label>Access Key</label>
                <input type="text" v-model="s3Config.accessKey" class="form-input" placeholder="AKIA..." />
              </div>
              <div class="form-group" style="flex:1;">
                <label>Secret Key</label>
                <input type="password" v-model="s3Config.secretKey" class="form-input" />
              </div>
            </div>

            <div v-if="testResult" class="test-result" :class="testResult.ok ? 'test-ok' : 'test-fail'">
              {{ testResult.ok ? 'Connection successful' : testResult.error }}
            </div>

            <div class="form-actions">
              <button class="btn btn-test" @click="handleTestConnection" :disabled="testLoading">
                {{ testLoading ? 'Testing...' : 'Test Connection' }}
              </button>
              <button class="btn btn-primary" @click="saveS3Config">Save & Sync</button>
            </div>
          </div>
        </div>
      </div>

      <!-- No file selected -->
      <div v-else-if="!activeFilePath" class="nb-empty-editor">
        <p>Select a notebook or create a new one</p>
      </div>

      <!-- Markdown Editor View -->
      <div v-else class="nb-editor-view">
        <div class="editor-header">
          <div class="file-path">
            {{ activeFilePath }}
          </div>
          <div class="editor-status">
            <div style="display: flex; align-items: center; gap: 6px;">
              <span class="status-dot" :class="{ saving: saving, unsynced: !synced }"></span>
              {{ saving ? 'Saving...' : synced ? 'Synced' : 'Unsaved' }}
            </div>
            <button class="nb-delete-btn" @click="handleDeleteFile" title="Delete file">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
            </button>
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

/* Drag-and-drop feedback */
.nb-folder.drag-over {
  background: rgba(79, 142, 247, 0.08);
  border-radius: 4px;
}
.nb-folder.drag-over > .nb-folder-name {
  color: #4f8ef7;
}
.nb-file[draggable="true"] {
  cursor: grab;
}
.nb-file[draggable="true"]:active {
  cursor: grabbing;
  opacity: 0.5;
}

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
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-test { background: transparent; color: #ccc; border: 1px solid #444; }
.btn-test:hover:not(:disabled) { background: #333; }
.btn-primary { background: #0e639c; color: white; }
.btn-primary:hover { background: #1177bb; }

.test-result { padding: 8px 12px; border-radius: 4px; font-size: 12px; margin-top: 4px; }
.test-ok { background: rgba(62,207,142,0.1); color: #3ecf8e; border: 1px solid rgba(62,207,142,0.2); }
.test-fail { background: rgba(239,68,68,0.1); color: #ef4444; border: 1px solid rgba(239,68,68,0.2); }

/* Loading / empty states */
.nb-loading, .nb-empty {
  padding: 24px 16px;
  font-size: 12px;
  color: #666;
  text-align: center;
}
.nb-empty p { margin-bottom: 4px; }

.nb-empty-editor {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #555;
  font-size: 14px;
}

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
.editor-status { font-size: 12px; color: #666; display: flex; align-items: center; gap: 16px; }
.status-dot { width: 6px; height: 6px; border-radius: 50%; background: #3ecf8e; transition: background 0.2s; }
.status-dot.saving { background: #f59e0b; }
.status-dot.unsynced { background: #666; }

.nb-delete-btn {
  background: transparent; border: none; color: #666; cursor: pointer;
  padding: 4px; border-radius: 4px; display: flex; align-items: center;
}
.nb-delete-btn:hover { background: rgba(239,68,68,0.15); color: #ef4444; }

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
