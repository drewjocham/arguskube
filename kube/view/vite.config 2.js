import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  build: {
    // Output to the Go embed directory so `wails build` picks up the assets.
    outDir: resolve(__dirname, '../backend/view/dist'),
    emptyOutDir: true,
  },
})
