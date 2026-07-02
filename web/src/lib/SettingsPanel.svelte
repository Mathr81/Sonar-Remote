<script lang="ts">
  import Icon from "./Icon.svelte";
  import { settings, saveSettings } from "./settings.svelte";
  import type { ClassicChannel } from "./api";

  interface ChannelOpt {
    id: ClassicChannel;
    label: string;
    accent: string;
    icon: string;
  }
  interface Props {
    channels: ChannelOpt[];
    onclose: () => void;
  }
  let { channels, onclose }: Props = $props();

  type ToggleKey = "showPresets" | "showDevices" | "showApps" | "showChatMix";
  const sections: { key: ToggleKey; label: string; icon: string }[] = [
    { key: "showPresets", label: "Profils EQ", icon: "eq" },
    { key: "showDevices", label: "Périphériques", icon: "headphones" },
    { key: "showApps", label: "Applications", icon: "media" },
    { key: "showChatMix", label: "ChatMix", icon: "chat" },
  ];
</script>

<div class="backdrop" onpointerdown={onclose} role="presentation">
  <div
    class="sheet"
    onpointerdown={(e) => e.stopPropagation()}
    role="dialog"
    aria-modal="true"
    aria-label="Réglages d'affichage"
  >
    <header>
      <span class="title"><Icon name="settings" size={17} /> Affichage</span>
      <button class="close" onclick={onclose} aria-label="Fermer">
        <Icon name="close" size={18} />
      </button>
    </header>

    <div class="body">
      <p class="hint">Choisis ce qui s'affiche à l'écran.</p>

      <div class="group-label">Éléments</div>
      {#each sections as s (s.key)}
        <label class="row">
          <span class="ic"><Icon name={s.icon} size={16} /></span>
          <span class="name">{s.label}</span>
          <span class="switch">
            <input
              type="checkbox"
              bind:checked={settings[s.key]}
              onchange={saveSettings}
            />
            <span class="track"></span>
          </span>
        </label>
      {/each}

      <div class="group-label">Canaux</div>
      {#each channels as c (c.id)}
        <label class="row" style="--accent:{c.accent}">
          <span class="ic accent"><Icon name={c.icon} size={16} /></span>
          <span class="name">{c.label}</span>
          <span class="switch">
            <input
              type="checkbox"
              bind:checked={settings.channels[c.id]}
              onchange={saveSettings}
            />
            <span class="track"></span>
          </span>
        </label>
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
    z-index: 60;
    animation: fade 0.15s ease;
  }
  @keyframes fade {
    from {
      opacity: 0;
    }
  }
  .sheet {
    width: min(400px, 100%);
    max-height: 85dvh;
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
    flex: none;
  }
  .title {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    font-weight: 700;
    font-size: 0.9rem;
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
  .body {
    overflow-y: auto;
    padding: 0.4rem 0.6rem 0.8rem;
  }
  .hint {
    margin: 0.5rem 0.4rem 0.3rem;
    font-size: 0.78rem;
    color: var(--text-dim);
  }
  .group-label {
    margin: 0.8rem 0.4rem 0.2rem;
    font-size: 0.7rem;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--text-dim);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 0.7rem;
    min-height: 46px;
    padding: 0.3rem 0.5rem;
    border-radius: 10px;
    cursor: pointer;
    -webkit-tap-highlight-color: transparent;
  }
  .row:active {
    background: rgba(255, 255, 255, 0.04);
  }
  .ic {
    display: flex;
    color: var(--text-dim);
    flex: none;
  }
  .ic.accent {
    color: var(--accent);
  }
  .name {
    flex: 1;
    font-size: 0.9rem;
    color: #d7dce6;
  }
  .switch {
    position: relative;
    width: 42px;
    height: 24px;
    flex: none;
  }
  .switch input {
    position: absolute;
    opacity: 0;
    width: 100%;
    height: 100%;
    margin: 0;
    cursor: pointer;
  }
  .track {
    position: absolute;
    inset: 0;
    background: rgba(255, 255, 255, 0.16);
    border-radius: 999px;
    transition: background 0.15s;
    pointer-events: none;
  }
  .track::after {
    content: "";
    position: absolute;
    top: 3px;
    left: 3px;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: #fff;
    transition: transform 0.15s;
  }
  .switch input:checked + .track {
    background: #5b8cff;
  }
  .switch input:checked + .track::after {
    transform: translateX(18px);
  }
</style>
