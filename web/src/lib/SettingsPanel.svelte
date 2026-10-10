<script lang="ts">
  import Sheet from "./Sheet.svelte";
  import Icon from "./Icon.svelte";
  import { settings, saveSettings } from "./settings.svelte";
  import type { ClassicChannel } from "./api";

  interface Props {
    channels: { id: ClassicChannel; label: string; icon: string }[];
    onclose: () => void;
  }
  let { channels, onclose }: Props = $props();

  const sections = [
    { key: "showDevices", label: "Périphériques", icon: "headphones" },
    { key: "showPresets", label: "Profils d'égaliseur", icon: "eq" },
    { key: "showApps", label: "Applications", icon: "media" },
    { key: "showChatMix", label: "ChatMix", icon: "chat" },
  ] as const;
</script>

<Sheet title="Affichage" {onclose}>
  {#each sections as s (s.key)}
    <label>
      <Icon name={s.icon} size={17} />
      <span>{s.label}</span>
      <input type="checkbox" role="switch" bind:checked={settings[s.key]} onchange={saveSettings} />
    </label>
  {/each}
  <h3>Canaux</h3>
  {#each channels as c (c.id)}
    <label>
      <Icon name={c.icon} size={17} />
      <span>{c.label}</span>
      <input type="checkbox" role="switch" bind:checked={settings.channels[c.id]} onchange={saveSettings} />
    </label>
  {/each}
</Sheet>

<style>
  label {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    min-height: 48px;
    padding: 0 0.65rem;
    border-radius: 12px;
    color: var(--dim);
    cursor: pointer;
  }
  span {
    flex: 1;
    color: var(--ink);
  }
  h3 {
    margin: 0.9rem 0.65rem 0.2rem;
    font-size: 0.8rem;
    font-weight: 500;
    color: var(--dim);
  }
  /* A native checkbox drawn as a switch. */
  input {
    appearance: none;
    flex: none;
    width: 44px;
    height: 26px;
    margin: 0;
    padding: 3px;
    border-radius: 13px;
    background: var(--raise);
    cursor: pointer;
    transition: background-color 0.15s;
  }
  input::before {
    content: "";
    display: block;
    width: 20px;
    height: 20px;
    border-radius: 50%;
    background: var(--ink);
    transition: translate 0.15s;
  }
  input:checked {
    background: var(--signal);
  }
  input:checked::before {
    translate: 18px 0;
    background: var(--well);
  }
  input:focus-visible {
    outline: 2px solid var(--ink);
    outline-offset: 2px;
  }
</style>
