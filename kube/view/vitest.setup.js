import { beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

// Run before every test in every file. Each test gets a fresh Pinia, so
// stores don't leak state between tests.
beforeEach(() => {
  setActivePinia(createPinia())
})
