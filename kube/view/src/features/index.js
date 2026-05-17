// Feature boot — auto-discovers every src/features/<id>/manifest.js and
// registers it. Adding a new feature is one step: create the folder.
// No edits to this file, no edits to the shell.
//
// Manifests are loaded eagerly (the manifest is tiny — id + lazy
// component refs). The actual panel/tab components stay code-split
// because their `component` fields are `() => import(...)` thunks that
// Vite splits per dynamic import.

import { registerFeature } from './registry'

const manifests = import.meta.glob('./*/manifest.{js,ts}', { eager: true })
for (const path in manifests) {
  const mod = manifests[path]
  const manifest = mod.default || mod.manifest
  if (!manifest) {
    console.warn(`[features] ${path} has no default export — skipping`)
    continue
  }
  registerFeature(manifest)
}
