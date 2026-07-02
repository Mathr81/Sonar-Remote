<script lang="ts">
  import Icon from "./Icon.svelte";
  import type { OutputDevice } from "./api";

  interface Props {
    title: string;
    icon?: string;
    devices: OutputDevice[];
    currentId: string;
    onselect: (id: string) => void;
    onclose: () => void;
  }
  let { title, icon = "headphones", devices, currentId, onselect, onclose }: Props =
    $props();

  function pick(id: string) {
    if (id !== currentId) onselect(id);
    onclose();
  }
</script>

<div
  class="backdrop"
  onpointerdown={onclose}
  role="presentation"
>
  <div
    class="sheet"
    onpointerdown={(e) => e.stopPropagation()}
    role="dialog"
    aria-modal="true"
    aria-label={title}
  >
    <header>
      <span class="title"><Icon name={icon} size={17} /> {title}</span>
      <button class="close" onclick={onclose} aria-label="Fermer">
        <Icon name="close" size={18} />
      </button>
    </header>

    <div class="list">
      {#each devices as d (d.id)}
        <button class="row" class:active={d.id === currentId} onclick={() => pick(d.id)}>
          <span class="name">{d.name}</span>
          {#if d.id === currentId}
            <span class="tick"><Icon name="check" size={18} /></span>
          {/if}
        </button>
      {:else}
        <div class="empty">Aucun périphérique disponible</div>
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
    width: min(440px, 100%);
    max-height: 80dvh;
    display: flex;
    flex-direction: column;
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
    padding: 0.9rem 1rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.07);
  }
  .title {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    font-weight: 700;
    font-size: 0.9rem;
    letter-spacing: 0.03em;
    color: #e6e9ef;
  }
  .close {
    display: flex;
    padding: 0.35rem;
    border: none;
    background: transparent;
    color: var(--text-dim);
    cursor: pointer;
    border-radius: 8px;
  }
  .close:active {
    background: rgba(255, 255, 255, 0.08);
  }
  .list {
    overflow-y: auto;
    padding: 0.4rem;
  }
  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    width: 100%;
    min-height: 52px;
    padding: 0.6rem 0.85rem;
    border: none;
    background: transparent;
    color: #d7dce6;
    text-align: left;
    font-size: 0.9rem;
    border-radius: 10px;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
  }
  .row:active {
    background: rgba(255, 255, 255, 0.06);
  }
  .row.active {
    background: color-mix(in srgb, #5b8cff 16%, transparent);
    color: #fff;
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .tick {
    display: flex;
    color: #5b8cff;
    flex: none;
  }
  .empty {
    padding: 1.5rem;
    text-align: center;
    color: var(--text-dim);
    font-size: 0.85rem;
  }
</style>
