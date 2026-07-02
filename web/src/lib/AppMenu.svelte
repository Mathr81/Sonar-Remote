<script lang="ts">
  import Icon from "./Icon.svelte";
  import type { ClassicChannel } from "./api";

  interface ChannelOpt {
    id: ClassicChannel;
    label: string;
    accent: string;
    icon: string;
  }

  interface Props {
    appName: string;
    channels: ChannelOpt[];
    currentChannel: ClassicChannel;
    onselect: (id: ClassicChannel) => void;
    onclose: () => void;
  }
  let { appName, channels, currentChannel, onselect, onclose }: Props = $props();

  function pick(id: ClassicChannel) {
    if (id !== currentChannel) onselect(id);
    onclose();
  }
</script>

<div class="backdrop" onpointerdown={onclose} role="presentation">
  <div
    class="sheet"
    onpointerdown={(e) => e.stopPropagation()}
    role="dialog"
    aria-modal="true"
    aria-label="Déplacer {appName}"
  >
    <header>
      <span class="title">Déplacer <b>{appName}</b> vers…</span>
      <button class="close" onclick={onclose} aria-label="Fermer">
        <Icon name="close" size={18} />
      </button>
    </header>

    <div class="list">
      {#each channels as c (c.id)}
        <button
          class="row"
          class:active={c.id === currentChannel}
          style="--accent:{c.accent}"
          onclick={() => pick(c.id)}
        >
          <span class="ic"><Icon name={c.icon} size={18} /></span>
          <span class="name">{c.label}</span>
          {#if c.id === currentChannel}
            <span class="tick"><Icon name="check" size={18} /></span>
          {/if}
        </button>
      {/each}
    </div>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.55);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1rem;
    z-index: 50;
    animation: fade 0.15s ease;
  }
  @keyframes fade {
    from {
      opacity: 0;
    }
  }
  .sheet {
    width: min(360px, 100%);
    background: #131925;
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 16px;
    box-shadow: 0 24px 60px rgba(0, 0, 0, 0.55);
    overflow: hidden;
    animation: pop 0.16s cubic-bezier(0.2, 0.9, 0.3, 1.2);
  }
  @keyframes pop {
    from {
      transform: translateY(10px) scale(0.98);
      opacity: 0;
    }
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 0.9rem 1rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.07);
  }
  .title {
    font-size: 0.88rem;
    color: #e6e9ef;
  }
  .title b {
    font-weight: 700;
  }
  .close {
    display: flex;
    padding: 0.35rem;
    border: none;
    background: transparent;
    color: var(--text-dim);
    cursor: pointer;
    border-radius: 8px;
    flex: none;
  }
  .close:active {
    background: rgba(255, 255, 255, 0.08);
  }
  .list {
    padding: 0.4rem;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 0.7rem;
    width: 100%;
    min-height: 50px;
    padding: 0.5rem 0.85rem;
    border: none;
    background: transparent;
    color: #d7dce6;
    text-align: left;
    font-size: 0.92rem;
    border-radius: 10px;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
  }
  .row:active {
    background: rgba(255, 255, 255, 0.06);
  }
  .row.active {
    background: color-mix(in srgb, var(--accent) 16%, transparent);
    color: #fff;
  }
  .ic {
    display: flex;
    color: var(--accent);
    flex: none;
  }
  .name {
    flex: 1;
  }
  .tick {
    display: flex;
    color: var(--accent);
    flex: none;
  }
</style>
