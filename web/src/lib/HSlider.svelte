<script lang="ts">
  // A horizontal fader for the compact tray flyout. Value is 0..1 (left..right).
  // Mirrors VSlider.svelte but on the x-axis.
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

  function valueFromEvent(clientX: number): number {
    const r = track.getBoundingClientRect();
    const v = (clientX - r.left) / r.width;
    return Math.min(1, Math.max(0, v));
  }

  function onPointerDown(e: PointerEvent) {
    dragging = true;
    onactive(true);
    track.setPointerCapture(e.pointerId);
    update(e.clientX);
  }
  function onPointerMove(e: PointerEvent) {
    if (!dragging) return;
    update(e.clientX);
  }
  function onPointerUp(e: PointerEvent) {
    if (!dragging) return;
    dragging = false;
    try {
      track.releasePointerCapture(e.pointerId);
    } catch {}
    onactive(false);
  }
  function update(clientX: number) {
    const v = valueFromEvent(clientX);
    value = v;
    onchange(v);
  }

  const pct = $derived(Math.round(value * 100));
</script>

<div
  bind:this={track}
  class="track"
  class:dragging
  class:muted
  style="--accent:{accent}"
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
  <div class="fill" style="width:{value * 100}%"></div>
  <div class="handle" style="left:{value * 100}%"></div>
</div>

<style>
  .track {
    position: relative;
    flex: 1;
    height: 30px;
    display: flex;
    align-items: center;
    cursor: pointer;
    touch-action: none;
  }
  .track::before {
    content: "";
    position: absolute;
    left: 0;
    right: 0;
    height: 7px;
    border-radius: 4px;
    background: rgba(255, 255, 255, 0.07);
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.05);
  }
  .fill {
    position: absolute;
    left: 0;
    height: 7px;
    border-radius: 4px;
    background: linear-gradient(
      to right,
      color-mix(in srgb, var(--accent) 45%, transparent),
      var(--accent)
    );
    transition: width 0.08s linear;
  }
  .track.dragging .fill {
    transition: none;
  }
  .handle {
    position: absolute;
    top: 50%;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: #e9edf4;
    transform: translate(-50%, -50%);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.5);
    transition: left 0.08s linear;
  }
  .track.dragging .handle {
    transition: none;
    box-shadow: 0 2px 12px color-mix(in srgb, var(--accent) 60%, black);
  }
  .muted .fill {
    filter: grayscale(0.85) brightness(0.7);
  }
  .muted .handle {
    background: #aeb6c4;
  }
</style>
