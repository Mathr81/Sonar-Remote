<script lang="ts">
  // Compact one-line channel control for the tray flyout: icon + label + a
  // horizontal fader + % + mute. Carries the same optimistic-hold and throttled
  // write behaviour as ChannelStrip.svelte so dragging feels instant and never
  // snaps back before the poll confirms.
  import HSlider from "./HSlider.svelte";
  import Icon from "./Icon.svelte";
  import { setVolume, setMute, type ClassicChannel } from "./api";

  interface Props {
    id: ClassicChannel;
    label: string;
    accent: string;
    icon: string;
    volume: number;
    muted: boolean;
  }
  let { id, label, accent, icon, volume, muted }: Props = $props();

  const VOL_EPS = 0.02;
  const HOLD_MS = 3000;

  let local = $state(volume);
  let active = $state(false);
  let desiredVol: number | null = null;
  let desiredVolAt = 0;

  $effect(() => {
    const v = volume;
    if (active) return;
    if (desiredVol !== null) {
      if (Math.abs(v - desiredVol) <= VOL_EPS || performance.now() - desiredVolAt > HOLD_MS) {
        desiredVol = null;
        local = v;
      }
      return;
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

  function onActive(aOn: boolean) {
    if (!aOn) {
      markDesired(local);
      clearTimeout(timer);
      setVolume(id, local).catch(() => {});
      pending = null;
    }
    active = aOn;
  }

  function toggleMute() {
    const nv = !localMuted;
    localMuted = nv;
    desiredMuted = nv;
    desiredMutedAt = performance.now();
    setMute(id, nv).catch(() => {});
  }

  const pct = $derived(Math.round(local * 100));
</script>

<div class="row" style="--accent:{accent}">
  <span class="icon" class:muted={localMuted}><Icon name={icon} size={17} /></span>
  <div class="body">
    <div class="labelrow">
      <span class="label">{label}</span>
      <span class="pct" class:dragging={active}>{localMuted ? "—" : pct + "%"}</span>
    </div>
    <HSlider bind:value={local} {accent} muted={localMuted} onchange={send} onactive={onActive} />
  </div>
  <button
    class="mute"
    class:on={localMuted}
    onclick={toggleMute}
    aria-pressed={localMuted}
    aria-label={localMuted ? "Réactiver " + label : "Couper " + label}
  >
    <Icon name={localMuted ? "mute" : "volume"} size={16} />
  </button>
</div>

<style>
  .row {
    display: flex;
    align-items: center;
    gap: 0.85rem;
    padding: 0.5rem 1rem;
    border-radius: 12px;
    margin: 0 0.5rem;
    transition: background 0.15s;
  }
  .row:hover {
    background: rgba(255, 255, 255, 0.03);
  }
  .icon {
    display: flex;
    flex: none;
    width: 26px;
    height: 26px;
    align-items: center;
    justify-content: center;
    border-radius: 8px;
    color: var(--accent);
    background: color-mix(in srgb, var(--accent) 14%, transparent);
    transition: filter 0.15s, color 0.15s;
  }
  .icon.muted {
    color: var(--text-dim);
    background: rgba(255, 255, 255, 0.05);
  }
  .body {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .labelrow {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    line-height: 1;
    padding: 0 0.15rem;
  }
  .label {
    font-size: 0.82rem;
    font-weight: 600;
    letter-spacing: 0.02em;
    color: var(--text);
  }
  .pct {
    font-variant-numeric: tabular-nums;
    font-size: 0.72rem;
    color: var(--text-dim);
    transition: color 0.15s;
  }
  .pct.dragging {
    color: var(--accent);
  }
  .mute {
    flex: none;
    width: 34px;
    height: 34px;
    border-radius: 9px;
    border: 1px solid rgba(255, 255, 255, 0.1);
    background: rgba(255, 255, 255, 0.05);
    color: var(--text-dim);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.15s;
  }
  .mute:active {
    transform: scale(0.92);
  }
  .mute.on {
    background: color-mix(in srgb, #ef4444 20%, transparent);
    border-color: #ef4444;
    color: #ff6b6b;
  }
</style>
