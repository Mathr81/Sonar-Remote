<script lang="ts">
  // The one slider of the app: a vertical channel fader, or a horizontal one
  // (narrow screens, ChatMix). It owns the optimistic value and write throttling.
  interface Props {
    value: number; // what Sonar reports
    shown?: number; // what is displayed (bindable, so a readout can follow a drag)
    min?: number;
    max?: number;
    origin?: number; // where the fill starts; a centre origin also snaps to it
    horizontal?: boolean;
    off?: boolean;
    label: string;
    onset: (v: number) => void;
  }
  let {
    value,
    shown = $bindable(value),
    min = 0,
    max = 1,
    origin = min,
    horizontal = false,
    off = false,
    label,
    onset,
  }: Props = $props();

  let el: HTMLDivElement;
  let travel: HTMLDivElement;
  let dragging = $state(false);
  let held = $state(false);
  let holdTimer: ReturnType<typeof setTimeout>;
  let sendTimer: ReturnType<typeof setTimeout>;
  let lastSent = 0;

  // Follow Sonar, except right after the user set a value: theirs wins until
  // Sonar has had time to apply it, so the cap never jumps back mid-gesture.
  $effect(() => {
    if (!dragging && !held) shown = value;
  });

  function set(v: number) {
    if (origin !== min && Math.abs(v - origin) < (max - min) * 0.02) v = origin;
    shown = Math.min(max, Math.max(min, v));
    held = true;
    clearTimeout(holdTimer);
    holdTimer = setTimeout(() => (held = false), 1500);

    // At most ~14 writes a second, always ending on the latest value.
    clearTimeout(sendTimer);
    const send = () => {
      lastSent = performance.now();
      onset(shown);
    };
    const wait = 70 - (performance.now() - lastSent);
    if (wait <= 0) send();
    else sendTimer = setTimeout(send, wait);
  }

  function fromPointer(e: PointerEvent) {
    const r = travel.getBoundingClientRect();
    const t = horizontal ? (e.clientX - r.left) / r.width : 1 - (e.clientY - r.top) / r.height;
    set(min + t * (max - min));
  }

  function down(e: PointerEvent) {
    dragging = true;
    el.setPointerCapture(e.pointerId);
    fromPointer(e);
  }

  function key(e: KeyboardEvent) {
    const step = (max - min) / 50;
    const target: Record<string, number> = {
      ArrowUp: shown + step,
      ArrowRight: shown + step,
      ArrowDown: shown - step,
      ArrowLeft: shown - step,
      PageUp: shown + step * 5,
      PageDown: shown - step * 5,
      Home: min,
      End: max,
    };
    if (e.key in target) {
      e.preventDefault();
      set(target[e.key]);
    }
  }

  const pos = (v: number) => (v - min) / (max - min);
  const t = $derived(pos(shown));
  const o = $derived(pos(origin));
</script>

<div
  bind:this={el}
  class="fader"
  class:h={horizontal}
  class:off
  class:dragging
  class:centered={origin !== min}
  style="--t:{t}; --lo:{Math.min(t, o)}; --hi:{Math.max(t, o)}"
  role="slider"
  tabindex="0"
  aria-label={label}
  aria-orientation={horizontal ? "horizontal" : "vertical"}
  aria-valuemin={Math.round(min * 100)}
  aria-valuemax={Math.round(max * 100)}
  aria-valuenow={Math.round(shown * 100)}
  onpointerdown={down}
  onpointermove={(e) => dragging && fromPointer(e)}
  onpointerup={() => (dragging = false)}
  onlostpointercapture={() => (dragging = false)}
  onkeydown={key}
>
  <div class="fill"></div>
  <div bind:this={travel} class="travel"><div class="cap"></div></div>
</div>

<style>
  .fader {
    --cap: 30px; /* cap length along the travel axis */
    --pad: 4px;
    --range: calc(100% - var(--cap) - 2 * var(--pad)); /* distance the cap centre covers */
    position: relative;
    flex: 1;
    width: 64px;
    min-height: 120px;
    border-radius: 16px;
    background: var(--well);
    box-shadow:
      inset 0 2px 3px rgb(0 0 0 / 0.55),
      inset 0 0 0 1px rgb(255 255 255 / 0.04);
    touch-action: none;
    cursor: grab;
  }
  .fader.dragging {
    cursor: grabbing;
  }
  .fader:focus-visible {
    outline: 2px solid var(--ink);
    outline-offset: 3px;
  }
  /* Scale marks every 10 %, engraved beside the slot. */
  .fader:not(.h)::before {
    content: "";
    position: absolute;
    right: calc(100% + 7px);
    width: 7px;
    top: calc(var(--cap) / 2 + var(--pad));
    bottom: calc(var(--cap) / 2 + var(--pad));
    background: linear-gradient(var(--line) 1px, transparent 1px) 0 0 / 100% calc((100% - 1px) / 10);
  }
  .fill {
    position: absolute;
    left: var(--pad);
    right: var(--pad);
    bottom: var(--pad);
    height: calc(var(--cap) / 2 + var(--range) * var(--t));
    border-radius: 12px;
    background: var(--signal);
    transition: background-color 0.2s;
  }
  .travel {
    position: absolute;
    inset: calc(var(--cap) / 2 + var(--pad)) var(--pad);
  }
  .cap {
    position: absolute;
    left: 0;
    right: 0;
    bottom: calc(var(--t) * 100%);
    height: var(--cap);
    translate: 0 50%;
    border-radius: 10px;
    background: var(--ink);
    box-shadow:
      0 1px 0 rgb(255 255 255 / 0.5) inset,
      0 4px 10px rgb(0 0 0 / 0.5);
    transition: scale 0.15s;
  }
  /* Grip line across the cap. */
  .cap::after {
    content: "";
    position: absolute;
    inset: 50% 30% auto;
    height: 2px;
    translate: 0 -50%;
    border-radius: 1px;
    background: rgb(0 0 0 / 0.28);
  }
  .dragging .cap {
    scale: 1.06;
  }
  .off .fill {
    background: var(--raise);
  }
  .off .cap {
    background: var(--dim);
  }

  /* Horizontal layout: same parts, turned a quarter. */
  .fader.h {
    flex: 1 1 auto;
    width: auto;
    min-height: 0;
    height: 44px;
    border-radius: 14px;
  }
  .h .fill {
    top: var(--pad);
    right: auto;
    height: auto;
    width: calc(var(--cap) / 2 + var(--range) * var(--t));
    border-radius: 10px;
  }
  .h.centered .fill {
    left: calc(var(--pad) + var(--cap) / 2 + var(--range) * var(--lo));
    width: calc(var(--range) * (var(--hi) - var(--lo)));
    border-radius: 0;
  }
  .h .travel {
    inset: var(--pad) calc(var(--cap) / 2 + var(--pad));
  }
  .h .cap {
    inset: 0 auto 0 calc(var(--t) * 100%);
    width: var(--cap);
    height: auto;
    translate: -50% 0;
  }
  .h .cap::after {
    inset: 30% auto 30% 50%;
    width: 2px;
    height: auto;
    translate: -50% 0;
  }
  /* Centre detent of the ChatMix balance. */
  .h.centered::after {
    content: "";
    position: absolute;
    left: 50%;
    top: -6px;
    width: 2px;
    height: 4px;
    translate: -50% 0;
    border-radius: 1px;
    background: var(--dim);
  }

  @media (prefers-reduced-motion: reduce) {
    .cap,
    .fill {
      transition: none;
    }
  }
</style>
