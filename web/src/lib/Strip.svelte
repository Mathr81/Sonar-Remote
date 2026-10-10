<script lang="ts">
  import Fader from "./Fader.svelte";
  import Icon from "./Icon.svelte";
  import { setVolume, setMute, type ClassicChannel, type VolumeObject } from "./api";

  interface Props {
    id: ClassicChannel;
    label: string;
    icon: string;
    level: VolumeObject;
    row: boolean; // narrow screens: lay the strip out as a horizontal row
    preset?: string; // undefined hides the line
    apps?: { processId: number; name: string }[]; // undefined hides the list
    onapp: (processId: number, name: string) => void;
  }
  let { id, label, icon, level, row, preset, apps, onapp }: Props = $props();

  let shown = $state(0); // set by the fader

  // Same optimistic hold as the fader: a tap shows at once and outlives the
  // polls that still carry the previous state.
  let muted = $state(false);
  let held = $state(false);
  let timer: ReturnType<typeof setTimeout>;
  $effect(() => {
    if (!held) muted = level.muted;
  });
  function toggleMute() {
    muted = !muted;
    held = true;
    clearTimeout(timer);
    timer = setTimeout(() => (held = false), 1500);
    setMute(id, muted).catch(() => {});
  }
</script>

<section class="strip" class:row class:muted aria-label={label}>
  <h2><Icon name={icon} size={17} />{label}</h2>
  <output class="level">{Math.round(shown * 100)}</output>
  <Fader
    value={level.volume}
    bind:shown
    horizontal={row}
    off={muted}
    label="Volume {label}"
    onset={(v) => setVolume(id, v).catch(() => {})}
  />
  <button class="mute" aria-pressed={muted} aria-label="Couper {label}" onclick={toggleMute}>
    <Icon name={muted ? "mute" : "volume"} size={20} />
  </button>
  {#if preset !== undefined || apps}
    <div class="meta">
      {#if preset !== undefined}
        <p class="preset">{#if preset}<Icon name="eq" size={13} />{preset}{/if}</p>
      {/if}
      {#if apps}
        <ul class="apps">
          {#each apps as a (a.processId)}
            <li><button title="Déplacer {a.name}" onclick={() => onapp(a.processId, a.name)}>{a.name}</button></li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</section>

<style>
  .strip {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.6rem;
    min-width: 0;
    min-height: 0;
    padding: 1rem 0.5rem 0.9rem;
  }
  .strip + :global(.strip) {
    border-left: 1px solid var(--line);
  }
  h2 {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin: 0;
    font-size: 0.95rem;
    font-weight: 600;
    color: var(--dim);
  }
  .level {
    font-size: clamp(1.9rem, 5.2vh, 2.9rem);
    font-weight: 600;
    line-height: 1;
    letter-spacing: -0.02em;
    font-variant-numeric: tabular-nums;
    margin-bottom: 0.3rem;
    transition: color 0.2s;
  }
  .muted .level {
    color: var(--dim);
  }
  .mute {
    display: grid;
    place-items: center;
    flex: none;
    width: 64px;
    height: 46px;
    border: 0;
    border-radius: 14px;
    background: var(--raise);
    color: var(--ink);
    cursor: pointer;
    transition:
      background-color 0.15s,
      scale 0.1s;
  }
  .mute:active {
    scale: 0.95;
  }
  .mute[aria-pressed="true"] {
    background: var(--mute);
  }
  .meta {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.4rem;
    width: 100%;
    min-width: 0;
  }
  .preset {
    display: flex;
    align-items: center;
    gap: 0.3rem;
    max-width: 100%;
    height: 1.2rem;
    margin: 0;
    font-size: 0.78rem;
    color: var(--dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .apps {
    display: flex;
    flex-direction: column;
    gap: 4px;
    width: 100%;
    height: 5.4rem; /* fixed, so every strip's fader ends on the same line */
    margin: 0;
    padding: 0;
    list-style: none;
    overflow-y: auto;
  }
  .apps button {
    width: 100%;
    padding: 0.35rem 0.6rem;
    border: 0;
    border-radius: 8px;
    background: var(--raise);
    color: var(--ink);
    font: inherit;
    font-size: 0.78rem;
    text-align: left;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    cursor: pointer;
  }
  .apps button:active {
    background: var(--line);
  }

  /* Narrow screens: name and level on one line, fader and mute on the next. */
  .strip.row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: 0.5rem 0.75rem;
    padding: 0.8rem 1rem;
  }
  .strip.row + :global(.strip) {
    border-left: 0;
    border-top: 1px solid var(--line);
  }
  .row h2 {
    color: var(--ink);
  }
  .row .level {
    font-size: 1.25rem;
    margin: 0;
    color: var(--dim);
  }
  .row .mute {
    width: 52px;
    height: 44px;
  }
  .row .meta {
    grid-column: 1 / -1;
    flex-flow: row wrap;
    align-items: center;
  }
  .row .preset:empty {
    display: none;
  }
  .row .apps {
    flex-flow: row wrap;
    width: auto;
    height: auto;
  }
  .row .apps button {
    width: auto;
    max-width: 12rem;
  }
</style>
