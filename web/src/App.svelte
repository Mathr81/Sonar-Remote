<script lang="ts">
  import Strip from "./lib/Strip.svelte";
  import Fader from "./lib/Fader.svelte";
  import Picker from "./lib/Picker.svelte";
  import SettingsPanel from "./lib/SettingsPanel.svelte";
  import Icon from "./lib/Icon.svelte";
  import { settings } from "./lib/settings.svelte";
  import {
    fetchLevels,
    fetchRouting,
    setOutputDevice,
    setMicDevice,
    setChatMix,
    moveApp,
    type ClassicChannel,
    type Levels,
    type Routing,
  } from "./lib/api";

  const CHANNELS: { id: ClassicChannel; label: string; icon: string }[] = [
    { id: "Master", label: "Master", icon: "master" },
    { id: "game", label: "Jeu", icon: "game" },
    { id: "chatRender", label: "Chat", icon: "chat" },
    { id: "media", label: "Média", icon: "media" },
    { id: "aux", label: "Aux", icon: "aux" },
    { id: "chatCapture", label: "Micro", icon: "mic" },
  ];
  // Render channels share one physical output and are the ones apps play into.
  const RENDER = CHANNELS.filter((c) => ["game", "chatRender", "media", "aux"].includes(c.id));

  // Phones in portrait get horizontal rows instead of vertical strips.
  const query = matchMedia("(max-width: 700px)");
  let narrow = $state(query.matches);
  query.onchange = () => (narrow = query.matches);

  let levels = $state<Levels | null>(null);
  let routing = $state<Routing | null>(null);
  let online = $state(true);

  let settingsOpen = $state(false);
  let picker = $state<null | "output" | "mic">(null);
  let moving = $state<null | { pid: number; name: string; channel: ClassicChannel }>(null);

  // Choices shown at once, until the next routing fetch confirms them.
  let pendingOutput = $state<string | null>(null);
  let pendingMic = $state<string | null>(null);
  let pendingMoves = $state<Record<number, ClassicChannel>>({});

  async function settle(write: Promise<void>) {
    await write.catch(() => {});
    await new Promise((r) => setTimeout(r, 400)); // Sonar applies device changes asynchronously
    routing = await fetchRouting().catch(() => routing);
    pendingOutput = pendingMic = null;
    pendingMoves = {};
  }

  const outputId = $derived(pendingOutput ?? routing?.outputId ?? "");
  const micId = $derived(pendingMic ?? routing?.micId ?? "");
  const visible = $derived(CHANNELS.filter((c) => settings.channels[c.id]));

  function levelOf(l: Levels, id: ClassicChannel) {
    const v = id === "Master" ? l.volumes.masters.classic : l.volumes.devices[id]?.classic;
    return v ?? { volume: 0, muted: false };
  }

  function appsOn(id: ClassicChannel) {
    if (!RENDER.some((c) => c.id === id)) return [];
    return (routing?.apps ?? []).filter((a) => (pendingMoves[a.processId] ?? a.channel) === id);
  }

  function pickDevice(id: string) {
    if (picker === "mic") {
      pendingMic = id;
      settle(setMicDevice(id));
    } else {
      pendingOutput = id;
      settle(setOutputDevice(id));
    }
  }

  function moveTo(channel: string) {
    const target = routing?.channelDeviceId[channel as ClassicChannel];
    if (!moving || !target) return;
    pendingMoves[moving.pid] = channel as ClassicChannel;
    settle(moveApp(target, moving.pid));
  }

  // Levels every second; devices, presets and apps every fifth tick (they
  // change rarely and are much heavier). Nothing at all while the page is hidden.
  let ticks = 0;
  async function tick() {
    if (document.hidden) return;
    try {
      levels = await fetchLevels();
      if (ticks++ % 5 === 0) routing = await fetchRouting();
      online = true;
    } catch {
      online = false;
      ticks = 0;
    }
  }
  $effect(() => {
    tick();
    const timer = setInterval(tick, 1000);
    document.addEventListener("visibilitychange", tick);
    return () => {
      clearInterval(timer);
      document.removeEventListener("visibilitychange", tick);
    };
  });
</script>

<div class="app">
  <header>
    {#if online && routing && settings.showDevices}
      <button class="device" onclick={() => (picker = "output")}>
        <Icon name="headphones" size={16} />
        <span>{routing.outputs.find((d) => d.id === outputId)?.name ?? "Choisir une sortie"}</span>
        <Icon name="chevron" size={14} />
      </button>
      <button class="device" onclick={() => (picker = "mic")}>
        <Icon name="mic" size={16} />
        <span>{routing.mics.find((d) => d.id === micId)?.name ?? "Choisir un micro"}</span>
        <Icon name="chevron" size={14} />
      </button>
    {/if}
    {#if online && levels?.mode === "stream"}
      <p class="note">Sonar est en mode Streamer : ces curseurs règlent le mix Classic.</p>
    {/if}
    <button class="gear" onclick={() => (settingsOpen = true)} aria-label="Réglages d'affichage">
      <Icon name="settings" size={19} />
    </button>
  </header>

  {#if !online}
    <main class="offline">
      <h1>Sonar est introuvable</h1>
      <p>Lance SteelSeries GG sur le PC : la télécommande se reconnecte toute seule.</p>
    </main>
  {:else if levels}
    <main class="board" class:rows={narrow}>
      {#each visible as c (c.id)}
        <Strip
          id={c.id}
          label={c.label}
          icon={c.icon}
          level={levelOf(levels, c.id)}
          row={narrow}
          preset={settings.showPresets ? (routing?.presets[c.id] ?? "") : undefined}
          apps={settings.showApps ? appsOn(c.id) : undefined}
          onapp={(pid, name) => (moving = { pid, name, channel: c.id })}
        />
      {/each}
    </main>
    {#if settings.showChatMix && levels.chatMix}
      <footer>
        <span>ChatMix</span>
        <Icon name="game" size={18} />
        <Fader
          horizontal
          value={levels.chatMix.balance}
          min={-1}
          max={1}
          origin={0}
          label="ChatMix, du jeu vers le chat"
          onset={(b) => setChatMix(b).catch(() => {})}
        />
        <Icon name="chat" size={18} />
      </footer>
    {/if}
  {/if}
</div>

{#if picker && routing}
  <Picker
    title={picker === "mic" ? "Microphone" : "Sortie audio"}
    options={(picker === "mic" ? routing.mics : routing.outputs).map((d) => ({ id: d.id, label: d.name }))}
    current={picker === "mic" ? micId : outputId}
    empty="Aucun périphérique détecté."
    onpick={pickDevice}
    onclose={() => (picker = null)}
  />
{/if}

{#if moving}
  <Picker
    title="Envoyer {moving.name} vers"
    options={RENDER}
    current={moving.channel}
    empty=""
    onpick={moveTo}
    onclose={() => (moving = null)}
  />
{/if}

{#if settingsOpen}
  <SettingsPanel channels={CHANNELS} onclose={() => (settingsOpen = false)} />
{/if}

<style>
  .app {
    display: grid;
    grid-template-rows: auto 1fr auto;
    grid-template-columns: minmax(0, 1fr); /* never wider than the screen */
    height: 100dvh;
    padding: env(safe-area-inset-top) env(safe-area-inset-right) env(safe-area-inset-bottom)
      env(safe-area-inset-left);
  }
  header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-height: 52px;
    padding: 0.4rem 0.6rem 0.4rem 0.9rem;
    border-bottom: 1px solid var(--line);
  }
  .device {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    flex: 0 1 auto;
    min-width: 0;
    height: 36px;
    padding: 0 0.7rem;
    border: 0;
    border-radius: 10px;
    background: none;
    color: var(--dim);
    font: inherit;
    font-size: 0.88rem;
    cursor: pointer;
  }
  .device span {
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .device:active,
  .gear:active {
    background: var(--raise);
  }
  .note {
    margin: 0 0.5rem;
    font-size: 0.82rem;
    color: var(--signal);
  }
  .gear {
    display: grid;
    place-items: center;
    flex: none;
    width: 40px;
    height: 40px;
    margin-left: auto;
    border: 0;
    border-radius: 12px;
    background: none;
    color: var(--dim);
    cursor: pointer;
  }
  .board {
    display: grid;
    grid-auto-flow: column;
    grid-auto-columns: 1fr;
    min-height: 0;
    overflow-y: auto;
  }
  .board.rows {
    grid-auto-flow: row;
    grid-template-columns: minmax(0, 1fr);
    align-content: start;
  }
  footer {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.8rem 1.25rem 0.9rem;
    border-top: 1px solid var(--line);
    color: var(--dim);
  }
  footer span {
    margin-right: 0.5rem;
    font-size: 0.95rem;
    font-weight: 600;
  }
  .offline {
    align-self: center;
    justify-self: center;
    max-width: 26rem;
    padding: 2rem;
  }
  .offline h1 {
    margin: 0 0 0.5rem;
    font-size: 1.6rem;
    font-weight: 600;
    letter-spacing: -0.01em;
  }
  .offline p {
    margin: 0;
    line-height: 1.5;
    color: var(--dim);
    text-wrap: pretty;
  }
</style>
