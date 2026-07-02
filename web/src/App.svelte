<script lang="ts">
  import ChannelStrip from "./lib/ChannelStrip.svelte";
  import ChatMix from "./lib/ChatMix.svelte";
  import DevicePicker from "./lib/DevicePicker.svelte";
  import AppMenu from "./lib/AppMenu.svelte";
  import SettingsPanel from "./lib/SettingsPanel.svelte";
  import Icon from "./lib/Icon.svelte";
  import { settings } from "./lib/settings.svelte";
  import {
    fetchSnapshot,
    setOutputDevice,
    setMicDevice,
    moveApp,
    type ClassicChannel,
    type OutputDevice,
    type Snapshot,
  } from "./lib/api";

  // Render channels share one physical classic output and can receive apps.
  const RENDER_CHANNELS = new Set<ClassicChannel>([
    "game",
    "chatRender",
    "media",
    "aux",
  ]);

  const CHANNELS: {
    id: ClassicChannel;
    label: string;
    accent: string;
    icon: string;
    vad?: string;
  }[] = [
    { id: "Master", label: "Master", accent: "#5b8cff", icon: "master" },
    { id: "game", label: "Game", accent: "#34d399", icon: "game", vad: "game" },
    { id: "chatRender", label: "Chat", accent: "#38bdf8", icon: "chat", vad: "chatRender" },
    { id: "media", label: "Média", accent: "#f472b6", icon: "media", vad: "media" },
    { id: "aux", label: "Aux", accent: "#a78bfa", icon: "aux", vad: "aux" },
    { id: "chatCapture", label: "Micro", accent: "#fb923c", icon: "mic", vad: "chatCapture" },
  ];

  let snap = $state<Snapshot | null>(null);
  let online = $state(false);

  // Modals
  let settingsOpen = $state(false);
  let picker = $state<null | "output" | "mic">(null);
  let appMenu = $state<null | { pid: number; name: string; channel: ClassicChannel }>(
    null,
  );

  // Optimistic selections — reflect a choice instantly, before the poll confirms.
  let pendingOutput = $state<OutputDevice | null>(null);
  let pendingOutputAt = 0;
  let pendingMic = $state<OutputDevice | null>(null);
  let pendingMicAt = 0;
  // pid -> optimistic target channel (+ timestamp)
  let pendingMoves = $state<Record<number, { channel: ClassicChannel; at: number }>>({});

  $effect(() => {
    if (!snap) return;
    if (
      pendingOutput &&
      (snap.currentOutputId === pendingOutput.id ||
        performance.now() - pendingOutputAt > 4000)
    ) {
      pendingOutput = null;
    }
    if (
      pendingMic &&
      (snap.currentMicId === pendingMic.id || performance.now() - pendingMicAt > 4000)
    ) {
      pendingMic = null;
    }
    // Drop optimistic moves once the poll agrees (or after a timeout).
    const now = performance.now();
    let changed = false;
    const next = { ...pendingMoves };
    for (const [pidStr, mv] of Object.entries(pendingMoves)) {
      const pid = Number(pidStr);
      const app = snap.apps.find((a) => a.processId === pid);
      if ((app && app.channel === mv.channel) || now - mv.at > 5000) {
        delete next[pid];
        changed = true;
      }
    }
    if (changed) pendingMoves = next;
  });

  function volOf(s: Snapshot, id: ClassicChannel) {
    const o =
      id === "Master" ? s.volumes?.masters.classic : s.volumes?.devices[id]?.classic;
    return { volume: o?.volume ?? 0, muted: o?.muted ?? false };
  }

  function presetOf(s: Snapshot, vad?: string) {
    if (!vad) return undefined;
    return s.presets.find((p) => p.virtualAudioDevice === vad)?.name;
  }

  function deviceFor(s: Snapshot, id: ClassicChannel) {
    if (pendingOutput && RENDER_CHANNELS.has(id)) return pendingOutput.name;
    if (pendingMic && id === "chatCapture") return pendingMic.name;
    return s.deviceNames[id];
  }

  // Effective channel of an app, honoring an in-flight optimistic move.
  function appsFor(s: Snapshot, channel: ClassicChannel) {
    return s.apps
      .filter((a) => (pendingMoves[a.processId]?.channel ?? a.channel) === channel)
      .map((a) => ({ processId: a.processId, name: a.name }));
  }

  function openPicker(id: ClassicChannel) {
    picker = id === "chatCapture" ? "mic" : "output";
  }

  function chooseDevice(id: string) {
    if (picker === "mic") {
      const dev = snap?.captureDevices.find((d) => d.id === id);
      if (!dev) return;
      pendingMic = dev;
      pendingMicAt = performance.now();
      setMicDevice(id).catch(() => {});
    } else {
      const dev = snap?.outputDevices.find((d) => d.id === id);
      if (!dev) return;
      pendingOutput = dev;
      pendingOutputAt = performance.now();
      setOutputDevice(id).catch(() => {});
    }
  }

  function moveAppTo(channel: ClassicChannel) {
    if (!appMenu || !snap) return;
    const target = snap.channelDeviceId[channel];
    if (!target) return;
    pendingMoves = {
      ...pendingMoves,
      [appMenu.pid]: { channel, at: performance.now() },
    };
    moveApp(target, appMenu.pid).catch(() => {});
  }

  // Channel options offered when moving an app (routable render channels only).
  const APP_TARGETS = CHANNELS.filter((c) => RENDER_CHANNELS.has(c.id));

  // Channels the user chose to display (settings-driven).
  const visibleChannels = $derived(CHANNELS.filter((c) => settings.channels[c.id]));

  async function tick() {
    try {
      snap = await fetchSnapshot();
      online = true;
    } catch {
      online = false;
    }
  }

  $effect(() => {
    tick();
    const iv = setInterval(tick, 1000);
    return () => clearInterval(iv);
  });
</script>

<div class="app">
  <header class="topbar">
    <div class="brand">
      <span class="logo">SONAR</span>
      <span class="sub">Remote</span>
    </div>
    <div class="right">
      <div class="status" class:online>
        <span class="dot"></span>
        {online ? (snap?.mode === "stream" ? "Streamer" : "Connecté") : "Hors ligne"}
      </div>
      <button class="gear" onclick={() => (settingsOpen = true)} aria-label="Réglages d'affichage">
        <Icon name="settings" size={18} />
      </button>
    </div>
  </header>

  {#if snap}
    <div class="board" style="--cols:{Math.max(1, visibleChannels.length)}">
      {#each visibleChannels as c (c.id)}
        {@const v = volOf(snap, c.id)}
        <ChannelStrip
          id={c.id}
          label={c.label}
          accent={c.accent}
          icon={c.icon}
          volume={v.volume}
          muted={v.muted}
          preset={presetOf(snap, c.vad)}
          device={deviceFor(snap, c.id)}
          selectableDevice={(RENDER_CHANNELS.has(c.id) && snap.outputDevices.length > 0) ||
            (c.id === "chatCapture" && snap.captureDevices.length > 0)}
          onpickdevice={() => openPicker(c.id)}
          appsEnabled={RENDER_CHANNELS.has(c.id)}
          apps={appsFor(snap, c.id)}
          onmoveapp={(pid, name) => (appMenu = { pid, name, channel: c.id })}
          showPreset={settings.showPresets}
          showDevice={settings.showDevices}
          showApps={settings.showApps}
        />
      {/each}
    </div>
    {#if settings.showChatMix}
      <ChatMix balance={snap.chatMix?.balance ?? 0} />
    {/if}

    {#if picker}
      <DevicePicker
        title={picker === "mic" ? "Microphone (entrée)" : "Périphérique de sortie"}
        icon={picker === "mic" ? "mic" : "headphones"}
        devices={picker === "mic" ? snap.captureDevices : snap.outputDevices}
        currentId={picker === "mic"
          ? (pendingMic?.id ?? snap.currentMicId)
          : (pendingOutput?.id ?? snap.currentOutputId)}
        onselect={chooseDevice}
        onclose={() => (picker = null)}
      />
    {/if}

    {#if appMenu}
      <AppMenu
        appName={appMenu.name}
        channels={APP_TARGETS}
        currentChannel={appMenu.channel}
        onselect={moveAppTo}
        onclose={() => (appMenu = null)}
      />
    {/if}
  {:else}
    <div class="loading">
      {online ? "Chargement…" : "Connexion à Sonar…"}
    </div>
  {/if}

  {#if settingsOpen}
    <SettingsPanel channels={CHANNELS} onclose={() => (settingsOpen = false)} />
  {/if}
</div>

<style>
  .app {
    height: 100dvh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.7rem 1.25rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.06);
    flex: none;
  }
  .brand {
    display: flex;
    align-items: baseline;
    gap: 0.5rem;
  }
  .logo {
    font-weight: 800;
    letter-spacing: 0.22em;
    font-size: 1.05rem;
    background: linear-gradient(90deg, #5b8cff, #a78bfa);
    -webkit-background-clip: text;
    background-clip: text;
    color: transparent;
  }
  .sub {
    font-size: 0.72rem;
    letter-spacing: 0.18em;
    text-transform: uppercase;
    color: var(--text-dim);
  }
  .right {
    display: flex;
    align-items: center;
    gap: 0.9rem;
  }
  .status {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.78rem;
    color: var(--text-dim);
  }
  .gear {
    display: flex;
    padding: 0.4rem;
    border: none;
    background: transparent;
    color: var(--text-dim);
    border-radius: 9px;
    cursor: pointer;
    transition: color 0.15s, background 0.15s;
    -webkit-tap-highlight-color: transparent;
  }
  .gear:hover {
    color: #e6e9ef;
  }
  .gear:active {
    background: rgba(255, 255, 255, 0.08);
  }
  .status .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #6b7280;
    transition: background 0.2s, box-shadow 0.2s;
  }
  .status.online .dot {
    background: #34d399;
    box-shadow: 0 0 8px #34d399;
  }
  .board {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: repeat(var(--cols, 6), 1fr);
  }
  .loading {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-dim);
    letter-spacing: 0.05em;
  }
</style>
