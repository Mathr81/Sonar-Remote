<script lang="ts">
  // A vertical, touch-first fader. Value is 0..1 (bottom..top).
  interface Props {
    value: number;
    accent: string;
    muted?: boolean;
    onchange: (v: number) => void;
    onactive: (active: boolean) => void;
  }

  let { value = $bindable(), accent, muted = false, onchange, onactive }: Props =
    $props();

  let track: HTMLDivElement;
  let dragging = $state(false);

  function valueFromEvent(clientY: number): number {
    const r = track.getBoundingClientRect();
    const v = 1 - (clientY - r.top) / r.height;
    return Math.min(1, Math.max(0, v));
  }

  function onPointerDown(e: PointerEvent) {
    dragging = true;
    onactive(true);
    track.setPointerCapture(e.pointerId);
    update(e.clientY);
  }

  function onPointerMove(e: PointerEvent) {
    if (!dragging) return;
    update(e.clientY);
  }

  function onPointerUp(e: PointerEvent) {
    if (!dragging) return;
    dragging = false;
    try {
      track.releasePointerCapture(e.pointerId);
    } catch {}
    onactive(false);
  }

  function update(clientY: number) {
    const v = valueFromEvent(clientY);
    value = v;
    onchange(v);
  }

  const pct = $derived(Math.round(value * 100));
</script>

<div class="fader" class:muted style="--accent:{accent}">
  <div class="readout" class:dragging>{pct}<span>%</span></div>
  <div
    bind:this={track}
    class="track"
    class:dragging
    onpointerdown={onPointerDown}
    onpointermove={onPointerMove}
    onpointerup={onPointerUp}
    onpointercancel={onPointerUp}
    role="slider"
    aria-valuemin={0}
    aria-valuemax={100}
    aria-valuenow={pct}
    tabindex="0"
  >
    <div class="fill" style="height:{value * 100}%"></div>
    <div class="handle" style="bottom:{value * 100}%">
      <span></span>
    </div>
  </div>
</div>

<style>
  .fader {
    display: flex;
    flex-direction: column;
    align-items: center;
    flex: 1;
    min-height: 0;
    gap: 0.5rem;
    width: 100%;
  }
  .readout {
    font-variant-numeric: tabular-nums;
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--text-dim);
    letter-spacing: 0.02em;
    transition: color 0.15s, transform 0.15s;
  }
  .readout span {
    font-size: 0.65em;
    margin-left: 1px;
  }
  .readout.dragging {
    color: var(--accent);
    transform: scale(1.15);
  }
  .track {
    position: relative;
    flex: 1;
    min-height: 0;
    width: 46px;
    border-radius: 24px;
    background: rgba(255, 255, 255, 0.05);
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.06);
    cursor: pointer;
    touch-action: none;
    overflow: hidden;
    transition: box-shadow 0.15s;
  }
  .track.dragging {
    box-shadow: inset 0 0 0 1px var(--accent);
  }
  .fill {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    background: linear-gradient(
      to top,
      color-mix(in srgb, var(--accent) 85%, transparent),
      color-mix(in srgb, var(--accent) 40%, transparent)
    );
    transition: height 0.08s linear;
  }
  .track.dragging .fill {
    transition: none;
  }
  .handle {
    position: absolute;
    left: 50%;
    transform: translate(-50%, 50%);
    width: 54px;
    height: 30px;
    border-radius: 9px;
    background: #e9edf4;
    box-shadow: 0 3px 10px rgba(0, 0, 0, 0.45);
    display: flex;
    align-items: center;
    justify-content: center;
    transition: bottom 0.08s linear;
    pointer-events: none;
  }
  .track.dragging .handle {
    transition: none;
    box-shadow: 0 3px 16px color-mix(in srgb, var(--accent) 60%, black);
  }
  .handle span {
    display: block;
    width: 18px;
    height: 3px;
    border-radius: 2px;
    background: var(--accent);
  }
  .muted .fill {
    filter: grayscale(0.85) brightness(0.7);
  }
  .muted .handle span {
    background: var(--text-dim);
  }
</style>
