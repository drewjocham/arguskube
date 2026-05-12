/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

interface Window {
  go?: {
    pkg?: {
      App?: Record<string, (...args: any[]) => Promise<any>>
    }
  }
  runtime?: {
    EventsOn: (eventName: string, callback: (...args: any[]) => void) => () => void
    EventsEmit: (eventName: string, ...args: any[]) => void
  }
  __argus_API_BASE__?: string
}
