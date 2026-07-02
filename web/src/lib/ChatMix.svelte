<script lang="ts">
  import Icon from "./Icon.svelte";
  import { setChatMix } from "./api";

  interface Props {
    balance: number; // -1 (game) .. 1 (chat)
  }
  let { balance }: Props = $props();

  const EPS = 0.03;
  const HOLD_MS = 3000;

  let local = $state(balance);
  let active = $state(false);
  let track: HTMLDivElement;
  let desired: number | null = null;
  let desiredAt = 0;

  // Optimistic hold: keep the value the user set until the server confirms it,
  // so releasing the handle never snaps back to the previous balance.
  $effect(() => {
    const b = balance;
    if (active) return;
    if (desired !== null) {
      if (Math.abs(b - desired) <= EPS || performance.now() - desiredAt > HOLD_MS) {
        desired = null;
        local = b;
      }
      return;
    }
    local = b;
  });

  let lastSent = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let pending: number | null = null;

  function send(b: number) {
    desired = b;
    desiredAt = performance.now();
    const now = performance.now();
    if (now - lastSent >= 70) {
      lastSent = now;
      setChatMix(b).catch(() => {});
    } else {
      pending = b;
      clearTimeout(timer);
      timer = setTimeout(() => {
        lastSent = performance.now();
        if (pending != null) setChatMix(pending).catch(() => {});
        pending = null;
      }, 70);
    }
  }

  function fromEvent(clientX: number): number {
    const r = track.getBoundingClientRect();
    const t = (clientX - r.left) / r.width; // 0..1
    return Math.min(1, Math.max(-1, t * 2 - 1));
  }

  function down(e: PointerEvent) {
    active = true;
    track.setPointerCapture(e.pointerId);
    move(e);
  }
  function move(e: PointerEvent) {
    if (!active) return;
    local = fromEvent(e.clientX);
    send(local);
  }
  function up(e: PointerEvent) {
    if (!active) return;
    try {
      track.releasePointerCapture(e.pointerId);
    } catch {}
    clearTimeout(timer);
    desired = local;
    desiredAt = performance.now();
    setChatMix(local).catch(() => {});
    pending = null;
    active = false;
  }

  const posPct = $derived(((local + 1) / 2) * 100);
</script>

<div class="chatmix">
  <span class="cap">Chatmix</span>
  <span class="side game"><Icon name="game" size={18} /></span>
  <div
    bind:this={track}
    class="track"
    onpointerdown={down}
    onpointermove={move}
    onpointerup={up}
    onpointercancel={up}
    role="slider"
    aria-valuemin={-100}
    aria-valuemax={100}
    aria-valuenow={Math.round(local * 100)}
    tabindex="0"
  >
    <div class="center"></div>
    <div
      class="fill"
      style="left:{Math.min(posPct, 50)}%; right:{100 - Math.max(posPct, 50)}%"
    ></div>
    <div class="handle" style="left:{posPct}%"></div>
  </div>
  <span class="side chat"><Icon name="chat" size={18} /></span>
</div>

<style>
  .chatmix {
    display: grid;
    grid-template-columns: auto 1fr auto;
    align-items: center;
    gap: 0.75rem;
    padding: 0.55rem 1.25rem 0.7rem;
    background: rgba(255, 255, 255, 0.02);
    border-top: 1px solid rgba(255, 255, 255, 0.06);
  }
  .cap {
    grid-column: 1 / -1;
    text-align: center;
    font-size: 0.7rem;
    letter-spacing: 0.16em;
    text-transform: uppercase;
    color: var(--text-dim);
    margin-bottom: 0.1rem;
  }
  .side {
    display: flex;
    color: var(--text-dim);
  }
  .side.game {
    color: #34d399;
  }
  .side.chat {
    color: #38bdf8;
  }
  .track {
    position: relative;
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
    height: 6px;
    border-radius: 3px;
    background: rgba(255, 255, 255, 0.08);
  }
  .center {
    position: absolute;
    left: 50%;
    top: 50%;
    width: 2px;
    height: 14px;
    transform: translate(-50%, -50%);
    background: rgba(255, 255, 255, 0.25);
    border-radius: 1px;
  }
  .fill {
    position: absolute;
    height: 6px;
    border-radius: 3px;
    background: linear-gradient(to right, #34d399, #38bdf8);
    opacity: 0.55;
  }
  .handle {
    position: absolute;
    top: 50%;
    width: 20px;
    height: 20px;
    border-radius: 50%;
    background: #e9edf4;
    transform: translate(-50%, -50%);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.5);
  }
</style>
