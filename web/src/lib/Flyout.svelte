<script lang="ts">
  // Compact EarTrumpet-style mixer shown in the tray flyout (WebView2). Reuses
  // the same snapshot poll + API as the full UI, just laid out as a tight
  // vertical stack of one-line channels plus ChatMix.
  import ChannelRow from "./ChannelRow.svelte";
  import ChatMix from "./ChatMix.svelte";
  import { settings } from "./settings.svelte";
  import { fetchLevels, type ClassicChannel, type Levels } from "./api";

  const CHANNELS: {
    id: ClassicChannel;
    label: string;
    accent: string;
    icon: string;
  }[] = [
    { id: "Master", label: "Master", accent: "#5b8cff", icon: "master" },
    { id: "game", label: "Game", accent: "#34d399", icon: "game" },
    { id: "chatRender", label: "Chat", accent: "#38bdf8", icon: "chat" },
    { id: "media", label: "Média", accent: "#f472b6", icon: "media" },
    { id: "aux", label: "Aux", accent: "#a78bfa", icon: "aux" },
    { id: "chatCapture", label: "Micro", accent: "#fb923c", icon: "mic" },
  ];

  let snap = $state<Levels | null>(null);
  let online = $state(false);

  const visibleChannels = $derived(CHANNELS.filter((c) => settings.channels[c.id]));

  function volOf(s: Levels, id: ClassicChannel) {
    const o =
      id === "Master" ? s.volumes?.masters.classic : s.volumes?.devices[id]?.classic;
    return { volume: o?.volume ?? 0, muted: o?.muted ?? false };
  }

  async function tick() {
    try {
      snap = await fetchLevels();
      online = true;
    } catch {
      online = false;
    }
  }

  // Poll only while the flyout is actually on screen. When it's hidden the host
  // sets the WebView2 controller invisible → document.hidden → we stop polling
  // entirely (0 requests, 0 CPU), and refresh instantly when it reopens.
  $effect(() => {
    let iv: ReturnType<typeof setInterval> | undefined;
    const start = () => {
      if (iv) return;
      tick();
      iv = setInterval(tick, 1000);
    };
    const stop = () => {
      clearInterval(iv);
      iv = undefined;
    };
    const onVis = () => (document.hidden ? stop() : start());
    document.addEventListener("visibilitychange", onVis);
    if (!document.hidden) start();
    return () => {
      stop();
      document.removeEventListener("visibilitychange", onVis);
    };
  });
</script>

<div class="flyout">
  <header>
    <span class="brand"><span class="logo">SONAR</span><span class="sub">Remote</span></span>
    <span class="status" class:online>
      <span class="dot"></span>
      {online ? (snap?.mode === "stream" ? "Streamer" : "Connecté") : "Hors ligne"}
    </span>
  </header>

  {#if snap}
    <div class="rows">
      {#each visibleChannels as c (c.id)}
        {@const v = volOf(snap, c.id)}
        <ChannelRow
          id={c.id}
          label={c.label}
          accent={c.accent}
          icon={c.icon}
          volume={v.volume}
          muted={v.muted}
        />
      {/each}
    </div>
    {#if settings.showChatMix && snap.chatMix}
      <ChatMix balance={snap.chatMix.balance} />
    {/if}
  {:else}
    <div class="loading">{online ? "Chargement…" : "Connexion à Sonar…"}</div>
  {/if}
</div>

<style>
  .flyout {
    height: 100dvh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: radial-gradient(
        700px 320px at 50% -20%,
        #161d2b 0%,
        transparent 60%
      ),
      var(--bg);
  }
  header {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.75rem 1rem 0.6rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  }
  .brand {
    display: inline-flex;
    align-items: baseline;
    gap: 0.45rem;
  }
  .logo {
    font-weight: 800;
    letter-spacing: 0.2em;
    font-size: 0.95rem;
    background: linear-gradient(90deg, #5b8cff, #a78bfa);
    -webkit-background-clip: text;
    background-clip: text;
    color: transparent;
  }
  .sub {
    font-size: 0.62rem;
    letter-spacing: 0.16em;
    text-transform: uppercase;
    color: var(--text-dim);
  }
  .status {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.72rem;
    color: var(--text-dim);
  }
  .status .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: #6b7280;
    transition: background 0.2s, box-shadow 0.2s;
  }
  .status.online .dot {
    background: #34d399;
    box-shadow: 0 0 8px #34d399;
  }
  .rows {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    justify-content: space-evenly;
    padding: 0.4rem 0;
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
