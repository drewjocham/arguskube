<script setup lang="ts">
import { ref, computed } from 'vue'
import { useProfilesStore } from '../../stores/profiles'

const profiles = useProfilesStore()

const editingGroupId = ref('')
const editingGroupName = ref('')
const editingGroupDesc = ref('')

const editingVariantKey = ref('') // "groupId:variantId"
const editingVariantName = ref('')
const editingVariantDesc = ref('')
const editingVariantVersion = ref('')

const newGroupName = ref('')
const newGroupDesc = ref('')
const showNewGroup = ref(false)

const newVariantGroupId = ref('')
const newVariantName = ref('')
const newVariantDesc = ref('')
const newVariantVersion = ref('1.0')
const showNewVariant = ref(false)

const captureMsg = ref('')
let captureTimer: ReturnType<typeof setTimeout> | null = null

function showCaptureMsg(msg: string) {
  captureMsg.value = msg
  if (captureTimer) clearTimeout(captureTimer)
  captureTimer = setTimeout(() => { captureMsg.value = '' }, 2500)
}

function startEditGroup(g: { id: string; name: string; description: string }) {
  editingGroupId.value = g.id
  editingGroupName.value = g.name
  editingGroupDesc.value = g.description
}

function saveEditGroup() {
  if (!editingGroupId.value) return
  profiles.updateGroup(editingGroupId.value, {
    name: editingGroupName.value,
    description: editingGroupDesc.value,
  })
  editingGroupId.value = ''
}

function cancelEditGroup() {
  editingGroupId.value = ''
}

function startEditVariant(groupId: string, v: { id: string; name: string; description: string; version: string }) {
  editingVariantKey.value = `${groupId}:${v.id}`
  editingVariantName.value = v.name
  editingVariantDesc.value = v.description
  editingVariantVersion.value = v.version
}

function saveEditVariant() {
  if (!editingVariantKey.value) return
  const [gid, vid] = editingVariantKey.value.split(':')
  profiles.updateVariant(gid, vid, {
    name: editingVariantName.value,
    description: editingVariantDesc.value,
    version: editingVariantVersion.value,
  })
  editingVariantKey.value = ''
}

function cancelEditVariant() {
  editingVariantKey.value = ''
}

function addGroup() {
  if (!newGroupName.value.trim()) return
  profiles.createGroup(newGroupName.value, newGroupDesc.value)
  newGroupName.value = ''
  newGroupDesc.value = ''
  showNewGroup.value = false
}

function addVariant() {
  if (!newVariantGroupId.value || !newVariantName.value.trim()) return
  profiles.createVariant(newVariantGroupId.value, newVariantName.value, newVariantDesc.value, newVariantVersion.value)
  newVariantName.value = ''
  newVariantDesc.value = ''
  newVariantVersion.value = '1.0'
  showNewVariant.value = false
}

function doCapture(groupId: string, variantId: string) {
  profiles.captureToVariant(groupId, variantId)
  showCaptureMsg('Current settings captured ✓')
}

function doApply(variantId: string) {
  profiles.applyVariant(variantId)
  showCaptureMsg('Profile applied ✓')
}

function doDuplicate(groupId: string, variantId: string) {
  profiles.duplicateVariant(groupId, variantId)
  showCaptureMsg('Profile duplicated ✓')
}

const sortedGroups = computed(() => {
  return [...profiles.groups]
})

const isGroupActive = (gid: string) => gid === profiles.activeGroupId
const isVariantActive = (vid: string) => vid === profiles.activeVariantId

function confirmDeleteGroup(id: string) {
  if (confirm('Delete this profile group and all its variants?')) {
    profiles.deleteGroup(id)
  }
}

function confirmDeleteVariant(groupId: string, variantId: string) {
  const v = profiles.groups.find(g => g.id === groupId)?.variants.find(x => x.id === variantId)
  if (confirm(`Delete "${v?.name || variantId}"?`)) {
    profiles.deleteVariant(groupId, variantId)
  }
}
</script>

<template>
  <div class="profiles-manager">
    <div v-if="captureMsg" class="capture-toast">{{ captureMsg }}</div>

    <!-- Profile Groups -->
    <div class="pm-section">
      <div class="pm-section-header">
        <h3>Profile Groups</h3>
        <button class="pm-add-btn" @click="showNewGroup = !showNewGroup">
          {{ showNewGroup ? 'Cancel' : '+ New Group' }}
        </button>
      </div>

      <!-- New Group Form -->
      <div v-if="showNewGroup" class="pm-inline-form">
        <input v-model="newGroupName" type="text" placeholder="Group name..." class="pm-input" @keyup.enter="addGroup" />
        <input v-model="newGroupDesc" type="text" placeholder="Description (optional)..." class="pm-input" />
        <div class="pm-form-actions">
          <button class="pm-btn primary" @click="addGroup">Create</button>
          <button class="pm-btn" @click="showNewGroup = false">Cancel</button>
        </div>
      </div>

      <!-- No groups -->
      <div v-if="sortedGroups.length === 0" class="pm-empty">
        <p>No profile groups yet. Create one to start saving your workspace settings.</p>
      </div>

      <!-- Group List -->
      <div v-for="g in sortedGroups" :key="g.id" class="pm-group-card">
        <!-- Group Header -->
        <div v-if="editingGroupId !== g.id" class="pm-group-header">
          <div class="pm-group-info" :class="{ active: isGroupActive(g.id) }">
            <span class="pm-group-name">{{ g.name }}</span>
            <span v-if="g.description" class="pm-group-desc">{{ g.description }}</span>
            <span class="pm-group-count">{{ g.variants.length }} variant{{ g.variants.length !== 1 ? 's' : '' }}</span>
          </div>
          <div class="pm-group-actions">
            <button class="pm-icon-btn" title="Edit group" @click="startEditGroup(g)">✎</button>
            <button class="pm-icon-btn" title="Add variant" @click="newVariantGroupId = g.id; showNewVariant = true">+</button>
            <button class="pm-icon-btn danger" title="Delete group" @click="confirmDeleteGroup(g.id)">×</button>
          </div>
        </div>

        <!-- Group Edit Form -->
        <div v-else class="pm-group-header editing">
          <div class="pm-inline-form">
            <input v-model="editingGroupName" type="text" class="pm-input" @keyup.enter="saveEditGroup" />
            <input v-model="editingGroupDesc" type="text" class="pm-input" placeholder="Description..." />
            <div class="pm-form-actions">
              <button class="pm-btn primary" @click="saveEditGroup">Save</button>
              <button class="pm-btn" @click="cancelEditGroup">Cancel</button>
            </div>
          </div>
        </div>

        <!-- Variants List -->
        <div v-if="g.variants.length > 0" class="pm-variants">
          <div
            v-for="v in g.variants"
            :key="v.id"
            class="pm-variant-row"
            :class="{ active: isVariantActive(v.id) && isGroupActive(g.id) }"
          >
            <!-- Variant View -->
            <div v-if="editingVariantKey !== `${g.id}:${v.id}`" class="pm-variant-info">
              <div class="pm-variant-main">
                <span class="pm-v-name">{{ v.name }}</span>
                <span class="pm-v-version">v{{ v.version }}</span>
                <span v-if="v.description" class="pm-v-desc">{{ v.description }}</span>
              </div>
              <div class="pm-variant-actions">
                <button class="pm-btn xs primary" title="Capture current settings" @click="doCapture(g.id, v.id)">
                  Capture
                </button>
                <button class="pm-btn xs" title="Apply this profile" @click="doApply(v.id)">
                  Apply
                </button>
                <button class="pm-icon-btn" title="Edit" @click="startEditVariant(g.id, v)">✎</button>
                <button class="pm-icon-btn" title="Duplicate" @click="doDuplicate(g.id, v.id)">⧉</button>
                <button class="pm-icon-btn danger" title="Delete" @click="confirmDeleteVariant(g.id, v.id)">×</button>
              </div>
            </div>

            <!-- Variant Edit Form -->
            <div v-else class="pm-variant-info editing">
              <div class="pm-inline-form compact">
                <input v-model="editingVariantName" type="text" class="pm-input" placeholder="Name..." @keyup.enter="saveEditVariant" />
                <input v-model="editingVariantVersion" type="text" class="pm-input pm-input-sm" placeholder="Version..." />
                <input v-model="editingVariantDesc" type="text" class="pm-input" placeholder="Description..." />
                <div class="pm-form-actions">
                  <button class="pm-btn primary" @click="saveEditVariant">Save</button>
                  <button class="pm-btn" @click="cancelEditVariant">Cancel</button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div v-else class="pm-no-variants">
          No variants yet.
        </div>

        <!-- New Variant Form -->
        <div v-if="showNewVariant && newVariantGroupId === g.id" class="pm-inline-form compact">
          <input v-model="newVariantName" type="text" placeholder="Variant name..." class="pm-input" @keyup.enter="addVariant" />
          <input v-model="newVariantDesc" type="text" placeholder="Description..." class="pm-input" />
          <input v-model="newVariantVersion" type="text" placeholder="Version..." class="pm-input pm-input-sm" />
          <div class="pm-form-actions">
            <button class="pm-btn primary" @click="addVariant">Create</button>
            <button class="pm-btn" @click="showNewVariant = false">Cancel</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.profiles-manager {
  padding: 12px 0;
}
.capture-toast {
  position: fixed;
  top: 56px;
  right: 24px;
  background: var(--accent);
  color: #fff;
  padding: 6px 14px;
  border-radius: 6px;
  font-size: 12px;
  z-index: 200;
  animation: fade-in 0.15s ease-out;
}
@keyframes fade-in { from { opacity: 0; transform: translateY(-4px); } to { opacity: 1; transform: translateY(0); } }

.pm-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.pm-section-header h3 {
  margin: 0;
  font-size: 13.5px;
  font-weight: 600;
  color: var(--text);
}
.pm-add-btn {
  background: none;
  border: 1px solid var(--border);
  color: var(--text2);
  padding: 4px 10px;
  border-radius: 5px;
  font-size: 11.5px;
  cursor: pointer;
  transition: all 0.12s;
}
.pm-add-btn:hover { background: var(--bg3); color: var(--text); border-color: var(--accent); }

.pm-empty {
  padding: 20px;
  text-align: center;
  color: var(--text3);
  font-size: 12px;
  background: var(--bg2);
  border-radius: 6px;
  border: 1px dashed var(--border);
}
.pm-empty p { margin: 0; }

.pm-group-card {
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 6px;
  margin-bottom: 10px;
  overflow: hidden;
}
.pm-group-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  gap: 8px;
}
.pm-group-header.editing {
  padding: 8px 12px;
}
.pm-group-info {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: baseline;
  gap: 8px;
}
.pm-group-info.active .pm-group-name {
  color: var(--accent);
}
.pm-group-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
  white-space: nowrap;
}
.pm-group-desc {
  font-size: 11.5px;
  color: var(--text3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pm-group-count {
  font-size: 10.5px;
  color: var(--text3);
  font-family: var(--mono);
  margin-left: auto;
  flex-shrink: 0;
}
.pm-group-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.pm-variants {
  border-top: 1px solid var(--border);
}
.pm-variant-row {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border);
  gap: 8px;
  transition: background 0.1s;
}
.pm-variant-row:last-child { border-bottom: none; }
.pm-variant-row.active {
  background: rgba(79,142,247,0.06);
  border-left: 2px solid var(--accent);
}
.pm-variant-info {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
}
.pm-variant-info.editing {
  flex-direction: column;
  align-items: stretch;
}
.pm-variant-main {
  display: flex;
  align-items: baseline;
  gap: 6px;
  min-width: 0;
}
.pm-v-name {
  font-size: 12.5px;
  font-weight: 500;
  color: var(--text);
}
.pm-v-version {
  font-size: 10.5px;
  font-family: var(--mono);
  color: var(--text3);
  padding: 1px 5px;
  border-radius: 3px;
  background: var(--bg3);
  flex-shrink: 0;
}
.pm-v-desc {
  font-size: 11px;
  color: var(--text3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pm-variant-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.pm-no-variants {
  padding: 10px 12px;
  font-size: 11.5px;
  color: var(--text3);
  border-top: 1px solid var(--border);
  font-style: italic;
}

.pm-inline-form {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 4px 0;
}
.pm-inline-form.compact {
  padding: 0;
}
.pm-input {
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text);
  border-radius: 4px;
  padding: 5px 8px;
  font-size: 12px;
  font-family: inherit;
}
.pm-input:focus { outline: none; border-color: var(--accent); }
.pm-input-sm { max-width: 100px; }
.pm-form-actions {
  display: flex;
  gap: 6px;
}
.pm-btn {
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text2);
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 11px;
  cursor: pointer;
  transition: all 0.12s;
  font-family: inherit;
}
.pm-btn:hover { background: var(--bg4); color: var(--text); }
.pm-btn.primary { background: var(--accent); color: #fff; border-color: var(--accent); }
.pm-btn.primary:hover { opacity: 0.9; }
.pm-btn.xs { padding: 2px 7px; font-size: 10.5px; }

.pm-icon-btn {
  background: none;
  border: 1px solid transparent;
  color: var(--text3);
  width: 24px;
  height: 24px;
  border-radius: 4px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  transition: all 0.12s;
}
.pm-icon-btn:hover { background: var(--bg3); color: var(--text); border-color: var(--border); }
.pm-icon-btn.danger:hover { background: rgba(240,84,84,0.12); color: var(--red); border-color: rgba(240,84,84,0.25); }
</style>
