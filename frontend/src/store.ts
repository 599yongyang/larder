import { create } from 'zustand'
import type { Settings } from './types'

const initialState = {
  schemaVersion: '1',
  cached: false,
  scanning: false,
  snapshot: { schemaVersion: '1', status: 'idle', roots: [], skills: [], agentLinks: [], issues: [], fromCache: false },
}

type AppStore = {
  state: any
  settings: Settings
  setState: (state: any) => void
  setSettings: (settings: Settings) => void
}

export const useAppStore = create<AppStore>((set) => ({
  state: initialState,
  settings: { projectWorkspaces: [], ignoredAgents: [], showAgentBuiltins: false, usageStatsEnabled: false, autoCheckUpdates: true, updateCheckIntervalHours: 24, snapshotRetention: 50 },
  setState: (state) => set({ state }),
  setSettings: (settings) => set({ settings }),
}))
