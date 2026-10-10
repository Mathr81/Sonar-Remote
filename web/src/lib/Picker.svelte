<script lang="ts">
  // Choose one option from a list: output device, microphone, or an app's channel.
  import Sheet from "./Sheet.svelte";
  import Icon from "./Icon.svelte";

  interface Props {
    title: string;
    options: { id: string; label: string }[];
    current: string;
    empty: string; // shown when there is nothing to choose from
    onpick: (id: string) => void;
    onclose: () => void;
  }
  let { title, options, current, empty, onpick, onclose }: Props = $props();

  function pick(id: string) {
    if (id !== current) onpick(id);
    onclose();
  }
</script>

<Sheet {title} {onclose}>
  {#each options as o (o.id)}
    <button aria-pressed={o.id === current} onclick={() => pick(o.id)}>
      <span>{o.label}</span>
      {#if o.id === current}<Icon name="check" size={18} />{/if}
    </button>
  {:else}
    <p>{empty}</p>
  {/each}
</Sheet>

<style>
  button {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    width: 100%;
    min-height: 50px;
    padding: 0.5rem 0.75rem;
    border: 0;
    border-radius: 12px;
    background: none;
    color: var(--ink);
    font: inherit;
    text-align: left;
    cursor: pointer;
  }
  button:active {
    background: var(--raise);
  }
  button[aria-pressed="true"] {
    background: var(--raise);
    color: var(--signal);
  }
  span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  p {
    margin: 0;
    padding: 1.25rem 0.75rem;
    color: var(--dim);
  }
</style>
