// Reactive, localStorage-backed display preferences. Shared across components:
// mutate `settings.*` then call saveSettings() to persist.
import type { ClassicChannel } from "./api";

export interface Settings {
  showPresets: boolean;
  showDevices: boolean;
  showApps: boolean;
  showChatMix: boolean;
  channels: Record<ClassicChannel, boolean>;
}

const KEY = "sonar-remote.settings.v1";

const DEFAULTS: Settings = {
  showPresets: true,
  showDevices: true,
  showApps: true,
  showChatMix: true,
  channels: {
    Master: true,
    game: true,
    chatRender: true,
    media: true,
    aux: true,
    chatCapture: true,
  },
};

function load(): Settings {
  try {
    const raw = JSON.parse(localStorage.getItem(KEY) || "{}");
    return {
      ...DEFAULTS,
      ...raw,
      channels: { ...DEFAULTS.channels, ...(raw.channels || {}) },
    };
  } catch {
    return structuredClone(DEFAULTS);
  }
}

export const settings = $state<Settings>(load());

export function saveSettings() {
  try {
    localStorage.setItem(KEY, JSON.stringify(settings));
  } catch {}
}
