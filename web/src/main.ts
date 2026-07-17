import { mount } from "svelte";
import "./app.css";
import App from "./App.svelte";
import Flyout from "./lib/Flyout.svelte";

// The Go server serves index.html for any path, so /flyout (loaded by the
// desktop tray's WebView2) reuses this same bundle but mounts the compact mixer.
const isFlyout = location.pathname.replace(/\/+$/, "").endsWith("/flyout");

const app = mount(isFlyout ? Flyout : App, {
  target: document.getElementById("app")!,
});

export default app;
