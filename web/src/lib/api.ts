// Thin client over the Go proxy. Every call hits /api/* which the local
// server forwards to Sonar's rotating port (re-discovering it on failure).

const BASE = "/api";

export type ClassicChannel =
  | "Master"
  | "game"
  | "chatRender"
  | "media"
  | "aux"
  | "chatCapture";

export interface VolumeObject {
  volume: number;
  muted: boolean;
}

export interface ClassicVolumes {
  masters: { classic?: VolumeObject };
  devices: Record<string, { classic?: VolumeObject }>;
}

export interface ChatMix {
  balance: number;
  state: string;
}

export interface SelectedConfig {
  name: string;
  virtualAudioDevice: string;
}

export interface Redirection {
  id: string;
  deviceId: string;
  isRunning: boolean;
}

export interface AudioDevice {
  id: string;
  friendlyName: string;
  dataFlow: "render" | "capture";
  role: string;
  isVad: boolean;
}

export interface AudioSession {
  processId: number;
  processName: string;
  displayName: string;
  isSystemSound: boolean;
  state: string; // active | inactive | expired
}

export interface DeviceRouting {
  deviceId: string;
  role: string;
  dataFlow: string;
  audioSessions: AudioSession[];
}

export interface OutputDevice {
  id: string;
  name: string;
}

/** An application currently routed to one of the render channels. */
export interface RoutedApp {
  processId: number;
  name: string;
  channel: ClassicChannel;
}

/** Full snapshot of everything the UI renders, fetched in one poll tick. */
export interface Snapshot {
  online: boolean;
  mode: string;
  volumes: ClassicVolumes | null;
  chatMix: ChatMix | null;
  presets: SelectedConfig[];
  deviceNames: Record<string, string>; // classic channel id -> friendly name
  outputDevices: OutputDevice[]; // selectable physical render outputs
  currentOutputId: string; // shared classic render output deviceId
  captureDevices: OutputDevice[]; // selectable physical mic inputs
  currentMicId: string; // classic mic input deviceId
  apps: RoutedApp[]; // apps routed to render channels
  channelDeviceId: Partial<Record<ClassicChannel, string>>; // channel -> Sonar VAD id
}

// Render channels share a single classic output; any of these carries its id.
const RENDER_REDIRECT_IDS = ["game", "media", "chat", "aux"];

// Sonar virtual-device role -> our channel id (routable render channels).
const ROLE_TO_CHANNEL: Record<string, ClassicChannel> = {
  game: "game",
  chatRender: "chatRender",
  media: "media",
  aux: "aux",
};

const STATE_PRIORITY: Record<string, number> = { active: 3, inactive: 2 };

async function getJSON<T>(path: string): Promise<T> {
  const r = await fetch(BASE + path, { cache: "no-store" });
  if (!r.ok) throw new Error(`${path} -> ${r.status}`);
  return (await r.json()) as T;
}

async function put(path: string): Promise<void> {
  const r = await fetch(BASE + path, { method: "PUT" });
  if (!r.ok) throw new Error(`${path} -> ${r.status}`);
}

// classicRedirections uses short ids; map them to our channel ids.
const REDIRECT_ID_TO_CHANNEL: Record<string, ClassicChannel> = {
  game: "game",
  chat: "chatRender",
  media: "media",
  aux: "aux",
  mic: "chatCapture",
};

export async function fetchSnapshot(): Promise<Snapshot> {
  const [mode, volumes, chatMix, presets, redirects, devices, routing] =
    await Promise.all([
      getJSON<string>("/mode").catch(() => "classic"),
      getJSON<ClassicVolumes>("/volumeSettings/classic"),
      getJSON<ChatMix>("/chatMix").catch(() => null),
      getJSON<SelectedConfig[]>("/configs/selected").catch(() => []),
      getJSON<Redirection[]>("/classicRedirections").catch(() => []),
      getJSON<AudioDevice[]>("/audioDevices").catch(() => []),
      getJSON<DeviceRouting[]>("/AudioDeviceRouting").catch(() => []),
    ]);

  const byId = new Map(devices.map((d) => [d.id, d.friendlyName]));
  const deviceNames: Record<string, string> = {};
  for (const r of redirects) {
    const ch = REDIRECT_ID_TO_CHANNEL[r.id];
    const name = byId.get(r.deviceId);
    if (ch && name) deviceNames[ch] = name;
  }

  const outputDevices: OutputDevice[] = devices
    .filter((d) => d.dataFlow === "render" && !d.isVad)
    .map((d) => ({ id: d.id, name: d.friendlyName }));
  const currentOutputId =
    redirects.find((r) => RENDER_REDIRECT_IDS.includes(r.id))?.deviceId ?? "";

  const captureDevices: OutputDevice[] = devices
    .filter((d) => d.dataFlow === "capture" && !d.isVad)
    .map((d) => ({ id: d.id, name: d.friendlyName }));
  const currentMicId = redirects.find((r) => r.id === "mic")?.deviceId ?? "";

  // Sonar virtual render devices carry a role identifying their channel.
  const channelDeviceId: Partial<Record<ClassicChannel, string>> = {};
  for (const d of devices) {
    if (d.dataFlow === "render" && d.isVad && ROLE_TO_CHANNEL[d.role]) {
      channelDeviceId[ROLE_TO_CHANNEL[d.role]] = d.id;
    }
  }

  // An app can appear under several devices; keep the one whose session is most
  // "live" (active > inactive), dropping system sounds and closed (expired) apps.
  const best = new Map<number, { name: string; channel: ClassicChannel; pri: number }>();
  for (const r of routing) {
    const ch = ROLE_TO_CHANNEL[r.role];
    if (!ch || r.dataFlow !== "render") continue;
    for (const s of r.audioSessions) {
      if (s.isSystemSound || s.processId === 0 || s.state === "expired") continue;
      const pri = STATE_PRIORITY[s.state] ?? 1;
      const cur = best.get(s.processId);
      if (!cur || pri > cur.pri) {
        best.set(s.processId, { name: s.displayName || s.processName, channel: ch, pri });
      }
    }
  }
  const apps: RoutedApp[] = [...best.entries()].map(([processId, v]) => ({
    processId,
    name: v.name,
    channel: v.channel,
  }));

  return {
    online: true,
    mode,
    volumes,
    chatMix,
    presets,
    deviceNames,
    outputDevices,
    currentOutputId,
    captureDevices,
    currentMicId,
    apps,
    channelDeviceId,
  };
}

/** Minimal snapshot for the compact tray flyout: just faders + chatmix + status.
 * Three requests instead of fetchSnapshot's seven — the flyout has no device
 * pickers or app routing, so polling those every tick would be wasted work. */
export interface Levels {
  online: boolean;
  mode: string;
  volumes: ClassicVolumes | null;
  chatMix: ChatMix | null;
}

export async function fetchLevels(): Promise<Levels> {
  const [mode, volumes, chatMix] = await Promise.all([
    getJSON<string>("/mode").catch(() => "classic"),
    getJSON<ClassicVolumes>("/volumeSettings/classic"),
    getJSON<ChatMix>("/chatMix").catch(() => null),
  ]);
  return { online: true, mode, volumes, chatMix };
}

/** Change the classic-mode physical output device (shared by all render channels). */
export function setOutputDevice(deviceId: string): Promise<void> {
  return put(
    `/classicRedirections/render/deviceId/${encodeURIComponent(deviceId)}`,
  );
}

/** Change the classic-mode microphone input device. */
export function setMicDevice(deviceId: string): Promise<void> {
  return put(
    `/classicRedirections/mic/deviceId/${encodeURIComponent(deviceId)}`,
  );
}

/** Route an application (by PID) to a render channel's Sonar virtual device. */
export function moveApp(targetDeviceId: string, processId: number): Promise<void> {
  return put(
    `/AudioDeviceRouting/render/${encodeURIComponent(targetDeviceId)}/${processId}`,
  );
}

export function setVolume(channel: ClassicChannel, value: number): Promise<void> {
  const v = Math.min(1, Math.max(0, value));
  return put(`/volumeSettings/classic/${channel}/Volume/${v.toFixed(5)}`);
}

export function setMute(channel: ClassicChannel, muted: boolean): Promise<void> {
  return put(`/volumeSettings/classic/${channel}/Mute/${muted}`);
}

export function setChatMix(balance: number): Promise<void> {
  const b = Math.min(1, Math.max(-1, balance));
  return put(`/chatMix?balance=${b.toFixed(4)}`);
}
