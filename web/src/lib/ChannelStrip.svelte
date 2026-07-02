<script lang="ts">
  import VSlider from "./VSlider.svelte";
  import Icon from "./Icon.svelte";
  import { setVolume, setMute, type ClassicChannel } from "./api";

  interface Props {
    id: ClassicChannel;
    label: string;
    accent: string;
    icon: string;
    volume: number;
    muted: boolean;
    preset?: string;
    device?: string;
    selectableDevice?: boolean;
    onpickdevice?: () => void;
    appsEnabled?: boolean;
    apps?: { processId: number; name: string }[];
    onmoveapp?: (processId: number, name: string) => void;
    showPreset?: boolean;
    showDevice?: boolean;
    showApps?: boolean;
  }

  let {
    id,
    label,
    accent,
    icon,
    volume,
    muted,
    preset,
    device,
    selectableDevice = false,
    onpickdevice,
    appsEnabled = false,
    apps = [],
    onmoveapp,
    showPreset = true,
    showDevice = true,
    showApps = true,
  }: Props = $props();

  // ── Optimistic UI ──────────────────────────────────────────────────────────
  // We display what the user just did immediately and *hold* it: incoming polls
  // that still show the old value are ignored until the server confirms our
  // value (or a safety timeout passes). This kills the post-release rollback and
  // makes every action feel instant, before the API has answered.
  const VOL_EPS = 0.02; // Sonar rounds volumes; treat near-equal as confirmed
  const HOLD_MS = 3000; // give up holding after this (in case a write was lost)

  let local = $state(volume);
  let active = $state(false); // finger is on the fader right now
  let desiredVol: number | null = null;
  let desiredVolAt = 0;

  $effect(() => {
    const v = volume; // re-run every poll
    if (active) return; // never fight an active drag
    if (desiredVol !== null) {
      if (Math.abs(v - desiredVol) <= VOL_EPS) {
        desiredVol = null;
        local = v;
      } else if (performance.now() - desiredVolAt > HOLD_MS) {
        desiredVol = null;
        local = v;
      }
      return; // otherwise keep showing our optimistic value
    }
    local = v;
  });

  let localMuted = $state(muted);
  let desiredMuted: boolean | null = null;
  let desiredMutedAt = 0;

  $effect(() => {
    const m = muted;
    if (desiredMuted !== null) {
      if (m === desiredMuted || performance.now() - desiredMutedAt > HOLD_MS) {
        desiredMuted = null;
        localMuted = m;
      }
      return;
    }
    localMuted = m;
  });

  // ── Throttled volume writes (~14/s) with a trailing flush ───────────────────
  let lastSent = 0;
  let pending: number | null = null;
  let timer: ReturnType<typeof setTimeout> | undefined;

  function markDesired(v: number) {
    desiredVol = v;
    desiredVolAt = performance.now();
  }

  function send(v: number) {
    markDesired(v);
    const now = performance.now();
    if (now - lastSent >= 70) {
      lastSent = now;
      setVolume(id, v).catch(() => {});
    } else {
      pending = v;
      clearTimeout(timer);
      timer = setTimeout(() => {
        lastSent = performance.now();
        if (pending != null) setVolume(id, pending).catch(() => {});
        pending = null;
      }, 70);
    }
  }

  function onActive(a: boolean) {
    if (!a) {
      // Release: commit the final value and hold it (no rollback).
      markDesired(local);
      clearTimeout(timer);
      setVolume(id, local).catch(() => {});
      pending = null;
    }
    active = a;
  }

  function toggleMute() {
    const nv = !localMuted;
    localMuted = nv; // optimistic
    desiredMuted = nv;
    desiredMutedAt = performance.now();
    setMute(id, nv).catch(() => {});
  }
</script>

<section class="strip" style="--accent:{accent}">
  <header>
    <span class="title"><Icon name={icon} size={18} />{label}</span>
  </header>

  {#if showPreset}
    <div class="meta preset" class:empty={!preset}>
      {#if preset}
        <Icon name="eq" size={13} /><span>{preset}</span>
      {/if}
    </div>
  {/if}

  {#if showDevice}
    {#if device && selectableDevice}
      <button class="meta device tappable" title={device} onclick={onpickdevice}>
        <Icon name="headphones" size={13} /><span>{device}</span>
        <Icon name="chevron" size={13} />
      </button>
    {:else}
      <div class="meta device" class:empty={!device} title={device ?? ""}>
        {#if device}
          <Icon name="headphones" size={13} /><span>{device}</span>
        {/if}
      </div>
    {/if}
  {/if}

  <VSlider bind:value={local} {accent} muted={localMuted} onchange={send} onactive={onActive} />

  <button
    class="mute"
    class:on={localMuted}
    onclick={toggleMute}
    aria-pressed={localMuted}
    aria-label={localMuted ? "Réactiver " + label : "Couper " + label}
  >
    <Icon name={localMuted ? "mute" : "volume"} size={18} />
  </button>

  {#if showApps}
    {#if appsEnabled}
      <div class="apps">
        {#each apps as a (a.processId)}
          <button class="chip" title={a.name} onclick={() => onmoveapp?.(a.processId, a.name)}>
            <span>{a.name}</span>
            <Icon name="chevron" size={12} />
          </button>
        {:else}
          <span class="apps-empty">Aucune app</span>
        {/each}
      </div>
    {:else}
      <div class="apps spacer"></div>
    {/if}
  {/if}
</section>

<style>
  .strip {
    display: flex;
    flex-direction: column;
    align-items: center;
    min-width: 0;
    padding: 0.75rem 0.4rem 0.9rem;
    border-right: 1px solid rgba(255, 255, 255, 0.05);
    min-height: 0;
  }
  header {
    height: 1.6rem;
    display: flex;
    align-items: center;
    margin-bottom: 0.6rem;
  }
  .title {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-weight: 700;
    font-size: 0.95rem;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--accent);
  }
  .meta {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    max-width: 100%;
    height: 1.35rem;
    padding: 0 0.5rem;
    border-radius: 7px;
    font-size: 0.72rem;
    color: var(--text-dim);
    background: rgba(255, 255, 255, 0.04);
    margin-bottom: 0.35rem;
  }
  .meta.empty {
    background: transparent;
  }
  .meta span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  button.meta {
    font: inherit;
    border: none;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    transition: background 0.15s;
  }
  .device.tappable span {
    flex: 1;
    text-align: left;
  }
  .device.tappable:active {
    background: rgba(255, 255, 255, 0.1);
  }
  .device.tappable :global(svg:last-child) {
    opacity: 0.55;
  }
  .meta :global(svg) {
    flex: none;
    opacity: 0.8;
  }
  .device {
    margin-bottom: 0.75rem;
  }
  .mute {
    margin-top: 0.85rem;
    width: 46px;
    height: 38px;
    border-radius: 11px;
    border: 1px solid rgba(255, 255, 255, 0.1);
    background: rgba(255, 255, 255, 0.05);
    color: var(--text-dim);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.15s;
    -webkit-tap-highlight-color: transparent;
  }
  .mute:active {
    transform: scale(0.92);
  }
  .mute.on {
    background: color-mix(in srgb, #ef4444 20%, transparent);
    border-color: #ef4444;
    color: #ff6b6b;
  }
  .apps {
    margin-top: 0.8rem;
    width: 100%;
    height: 96px;
    flex: none;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 5px;
    border-radius: 9px;
    background: rgba(255, 255, 255, 0.03);
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.05);
  }
  .apps.spacer {
    background: transparent;
    box-shadow: none;
  }
  .chip {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    width: 100%;
    padding: 5px 7px;
    border: none;
    border-left: 3px solid var(--accent);
    background: rgba(255, 255, 255, 0.06);
    color: #cfd6e2;
    font: inherit;
    font-size: 0.72rem;
    border-radius: 6px;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
    transition: background 0.12s;
  }
  .chip span {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: left;
  }
  .chip :global(svg) {
    opacity: 0.5;
    flex: none;
  }
  .chip:active {
    background: rgba(255, 255, 255, 0.13);
  }
  .apps-empty {
    margin: auto;
    color: var(--text-dim);
    font-size: 0.72rem;
    opacity: 0.5;
  }
</style>

