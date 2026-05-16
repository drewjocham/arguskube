<script setup>
import { ref, computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useGuidesStore } from '../../stores/guides'

const emit = defineEmits(['select-guide'])

const guides = useGuidesStore()
const { filteredGuides, categories, hasActiveFilter, searchQuery, activeCategory } = storeToRefs(guides)

const searchInput = ref('')

function onSearchInput() {
  guides.setSearchQuery(searchInput.value)
}

function onCategoryClick(catId) {
  guides.setActiveCategory(catId)
}

function onClearSearch() {
  searchInput.value = ''
  guides.setSearchQuery('')
}

function onClearFilters() {
  searchInput.value = ''
  guides.setActiveCategory(null)
}

const categoryList = computed(() => categories.value)
</script>

<template>
  <div class="guide-list">
    <div class="guide-header">
      <div class="guide-title-row">
        <h2 class="guide-title">Documentation</h2>
        <span class="guide-count">{{ filteredGuides.length }} guide{{ filteredGuides.length !== 1 ? 's' : '' }}</span>
      </div>
      <p class="guide-subtitle">Step-by-step walkthroughs for every feature. Pick a guide to start an interactive tour with live UI highlights.</p>
    </div>

    <div class="guide-search-row">
      <div class="guide-search-wrap">
        <svg class="search-icon" width="14" height="14" viewBox="0 0 14 14" fill="none">
          <circle cx="6" cy="6" r="4.5" stroke="currentColor" stroke-width="1.3"/>
          <path d="M9.5 9.5L13 13" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
        </svg>
        <input
          ref="searchInput"
          v-model="searchInput"
          type="text"
          class="guide-search"
          placeholder="Search guides by name, feature, or step..."
          @input="onSearchInput"
        />
        <button
          v-if="searchInput"
          class="search-clear"
          @click="onClearSearch"
          title="Clear search"
        >
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
            <path d="M3 3l6 6M9 3l-6 6" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
          </svg>
        </button>
      </div>
    </div>

    <div class="guide-categories">
      <button
        class="cat-pill"
        :class="{ active: !activeCategory }"
        @click="onClearFilters"
      >
        All
      </button>
      <button
        v-for="cat in categoryList"
        :key="cat.id"
        class="cat-pill"
        :class="{ active: activeCategory === cat.id }"
        @click="onCategoryClick(cat.id)"
      >
        {{ cat.label }}
        <span class="cat-count">{{ cat.count }}</span>
      </button>
    </div>

    <div class="guide-cards">
      <div v-if="filteredGuides.length === 0" class="guide-empty">
        <div class="guide-empty-icon">
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="11" cy="11" r="8"/>
            <path d="M21 21l-4.35-4.35"/>
          </svg>
        </div>
        <p class="guide-empty-text">No guides match your search.</p>
        <button class="guide-empty-btn" @click="onClearFilters">Clear filters</button>
      </div>

      <button
        v-for="guide in filteredGuides"
        :key="guide.sectionId"
        class="guide-card"
        @click="emit('select-guide', guide.sectionId)"
      >
        <div class="guide-card-icon">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path :d="guide.icon"/>
          </svg>
        </div>
        <div class="guide-card-body">
          <div class="guide-card-title">{{ guide.label }}</div>
          <div class="guide-card-meta">{{ guide.steps.length }} step{{ guide.steps.length !== 1 ? 's' : '' }}</div>
          <div class="guide-card-cats">
            <span
              v-for="catId in guide.categories"
              :key="catId"
              class="guide-card-cat"
            >
              {{ categories.find((c) => c.id === catId)?.label || catId }}
            </span>
          </div>
        </div>
        <svg class="guide-card-arrow" width="14" height="14" viewBox="0 0 14 14" fill="none">
          <path d="M5 3l4 4-4 4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>
    </div>
  </div>
</template>

<style scoped>
.guide-list {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 24px;
  overflow-y: auto;
  min-height: 0;
}

.guide-header {}

.guide-title-row {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 6px;
}

.guide-title {
  font-size: 20px;
  font-weight: 500;
  color: var(--text);
  margin: 0;
}

.guide-count {
  font-size: 12px;
  color: var(--text3);
  font-variant-numeric: tabular-nums;
}

.guide-subtitle {
  font-size: 13px;
  color: var(--text2);
  line-height: 1.5;
  margin: 0;
}

.guide-search-row {}

.guide-search-wrap {
  position: relative;
  display: flex;
  align-items: center;
}

.search-icon {
  position: absolute;
  left: 12px;
  color: var(--text3);
  pointer-events: none;
}

.guide-search {
  width: 100%;
  padding: 8px 34px 8px 34px;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  font: inherit;
  font-size: 13px;
  outline: none;
  transition: border-color 0.15s;
}

.guide-search::placeholder {
  color: var(--text3);
}

.guide-search:focus {
  border-color: var(--accent2);
}

.search-clear {
  position: absolute;
  right: 8px;
  background: none;
  border: none;
  color: var(--text3);
  cursor: pointer;
  padding: 4px;
  border-radius: 3px;
  display: flex;
  transition: all 0.1s;
}

.search-clear:hover {
  color: var(--text);
  background: var(--bg3);
}

.guide-categories {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.cat-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  border-radius: 14px;
  border: 1px solid var(--border);
  background: var(--bg3);
  color: var(--text2);
  font: inherit;
  font-size: 11.5px;
  cursor: pointer;
  transition: all 0.1s;
}

.cat-pill:hover {
  background: var(--bg4);
  color: var(--text);
}

.cat-pill.active {
  background: rgba(74, 158, 255, 0.12);
  border-color: rgba(74, 158, 255, 0.3);
  color: var(--accent2);
}

.cat-count {
  font-size: 10px;
  color: var(--text3);
  background: var(--bg2);
  padding: 1px 5px;
  border-radius: 8px;
  font-variant-numeric: tabular-nums;
}

.cat-pill.active .cat-count {
  background: rgba(74, 158, 255, 0.1);
  color: var(--accent2);
}

.guide-cards {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
  min-height: 0;
}

.guide-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.12s;
  text-align: left;
  font: inherit;
  color: inherit;
  width: 100%;
}

.guide-card:hover {
  background: var(--bg3);
  border-color: var(--accent2);
  transform: translateY(-1px);
}

.guide-card-icon {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  background: rgba(74, 158, 255, 0.08);
  color: var(--accent2);
  flex-shrink: 0;
}

.guide-card-body {
  flex: 1;
  min-width: 0;
}

.guide-card-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
  margin-bottom: 2px;
}

.guide-card-meta {
  font-size: 11px;
  color: var(--text3);
  margin-bottom: 6px;
}

.guide-card-cats {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.guide-card-cat {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 3px;
  background: var(--bg3);
  color: var(--text3);
  border: 1px solid var(--border2);
}

.guide-card-arrow {
  flex-shrink: 0;
  color: var(--text3);
  transition: color 0.1s;
}

.guide-card:hover .guide-card-arrow {
  color: var(--accent2);
}

.guide-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 40px 20px;
  text-align: center;
}

.guide-empty-icon {
  color: var(--text3);
  opacity: 0.6;
}

.guide-empty-text {
  font-size: 13px;
  color: var(--text3);
  margin: 0;
}

.guide-empty-btn {
  padding: 5px 14px;
  border-radius: 6px;
  border: 1px solid var(--border2);
  background: var(--bg3);
  color: var(--text2);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.1s;
}

.guide-empty-btn:hover {
  background: var(--bg4);
  color: var(--text);
}
</style>
