<script lang="ts">
  // A modal sheet on the native <dialog>: focus trap, Esc and backdrop for free.
  import type { Snippet } from "svelte";
  import Icon from "./Icon.svelte";

  interface Props {
    title: string;
    onclose: () => void;
    children: Snippet;
  }
  let { title, onclose, children }: Props = $props();

  let dialog: HTMLDialogElement;
  $effect(() => dialog.showModal());
</script>

<!-- The dialog has no padding, so a click landing on it is a click on the backdrop. -->
<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
<dialog
  bind:this={dialog}
  aria-label={title}
  {onclose}
  onclick={(e) => e.target === dialog && dialog.close()}
>
  <header>
    <h2>{title}</h2>
    <button onclick={() => dialog.close()} aria-label="Fermer"><Icon name="close" size={18} /></button>
  </header>
  <div class="body">{@render children()}</div>
</dialog>

<style>
  dialog {
    width: min(26rem, calc(100vw - 2rem));
    max-height: min(42rem, calc(100dvh - 2rem));
    padding: 0;
    border: 1px solid var(--line);
    border-radius: 20px;
    background: var(--desk);
    color: var(--ink);
    box-shadow: 0 24px 60px rgb(0 0 0 / 0.5);
    overflow: hidden;
  }
  dialog[open] {
    display: flex;
    flex-direction: column;
    animation: rise 0.18s ease-out;
  }
  dialog::backdrop {
    background: rgb(0 0 0 / 0.55);
  }
  @keyframes rise {
    from {
      opacity: 0;
      translate: 0 12px;
    }
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.9rem 0.9rem 0.6rem 1.25rem;
  }
  h2 {
    margin: 0;
    font-size: 1.05rem;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  header button {
    display: grid;
    place-items: center;
    flex: none;
    width: 36px;
    height: 36px;
    border: 0;
    border-radius: 10px;
    background: none;
    color: var(--dim);
    cursor: pointer;
  }
  header button:active {
    background: var(--raise);
  }
  .body {
    padding: 0 0.6rem 0.7rem;
    overflow-y: auto;
  }
  @media (prefers-reduced-motion: reduce) {
    dialog[open] {
      animation: none;
    }
  }
</style>
