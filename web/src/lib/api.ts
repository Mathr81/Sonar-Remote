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

/** The fast-moving part of Sonar's state, polled every second. */
export interface Levels {
  mode: string;
  volumes: ClassicVolumes;
  chatMix: ChatMix | null;
}

/** Devices, presets and app routing: changes rarely, polled less often. */
export interface Routing {
  presets: Record<string, string>; // channel id -> EQ preset name
  outputs: OutputDevice[]; // selectable physical render outputs
  outputId: string; // output shared by every render channel
  mics: OutputDevice[]; // selectable physical mic inputs
  micId: string;
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

export async function fetchLevels(): Promise<Levels> {
  const [mode, volumes, chatMix] = await Promise.all([
    getJSON<string>("/mode").catch(() => "classic"),
    getJSON<ClassicVolumes>("/volumeSettings/classic"),
    getJSON<ChatMix>("/v1/chatMix").catch(() => null),
  ]);
  return { mode, volumes, chatMix };
}

export async function fetchRouting(): Promise<Routing> {
  const [selected, redirects, devices, routing] = await Promise.all([
    getJSON<SelectedConfig[]>("/configs/selected").catch(() => []),
    getJSON<Redirection[]>("/classicRedirections").catch(() => []),
    getJSON<AudioDevice[]>("/audioDevices").catch(() => []),
    getJSON<DeviceRouting[]>("/AudioDeviceRouting").catch(() => []),
  ]);

  const physical = (flow: string): OutputDevice[] =>
    devices
      .filter((d) => d.dataFlow === flow && !d.isVad)
      .map((d) => ({ id: d.id, name: d.friendlyName }));

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

  return {
    presets: Object.fromEntries(selected.map((p) => [p.virtualAudioDevice, p.name])),
    outputs: physical("render"),
    outputId: redirects.find((r) => RENDER_REDIRECT_IDS.includes(r.id))?.deviceId ?? "",
    mics: physical("capture"),
    micId: redirects.find((r) => r.id === "mic")?.deviceId ?? "",
    apps: [...best].map(([processId, v]) => ({ processId, name: v.name, channel: v.channel })),
    channelDeviceId,
  };
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
  return put(`/v1/chatMix?balance=${b.toFixed(4)}`);
}
