<script>
  import { onMount } from "svelte";

  let loading = true;
  let error = "";
  let status = null;

  const trendBars = [42, 68, 56, 88, 73, 64, 92];

  onMount(async () => {
    try {
      const response = await fetch("/api/v1/status");
      if (!response.ok) {
        throw new Error(`status request failed: ${response.status}`);
      }

      status = await response.json();
    } catch (err) {
      error = err instanceof Error ? err.message : "unknown error";
    } finally {
      loading = false;
    }
  });

  $: cards = status
    ? [
        { label: "Service", value: status.service },
        { label: "Version", value: status.version },
        { label: "Config root", value: status.paths.configDir },
        { label: "Content root", value: status.paths.contentDir }
      ]
    : [];
</script>

<svelte:head>
  <title>dnsmanager foundation</title>
</svelte:head>

<main class="shell">
  <section class="hero">
    <div class="hero-copy">
      <p class="eyebrow">dnsmanager foundation</p>
      <h1>Control-plane scaffolding for a companion `dnsmasq` container.</h1>
      <p class="lede">
        This first runnable slice wires the Go backend, Cobra CLI, Svelte shell,
        and shared-volume Compose model together so later milestones can focus
        on DNS, DHCP, TFTP, PXE, and observability features.
      </p>
    </div>
    <div class="hero-panel">
      <div class="panel-title">Pi-hole-inspired dashboard shell</div>
      <div class="trend">
        {#each trendBars as height}
          <span style={`height:${height}%`}></span>
        {/each}
      </div>
      <div class="trend-labels">
        <span>queries</span>
        <span>clients</span>
        <span>leases</span>
      </div>
    </div>
  </section>

  <section class="status-grid">
    {#if loading}
      <article class="card muted">Loading backend status…</article>
    {:else if error}
      <article class="card error">
        <h2>Backend unavailable</h2>
        <p>{error}</p>
      </article>
    {:else}
      {#each cards as card}
        <article class="card">
          <p class="card-label">{card.label}</p>
          <h2>{card.value}</h2>
        </article>
      {/each}
    {/if}
  </section>

  <section class="columns">
    <article class="panel">
      <h2>Planned feature areas</h2>
      <ul>
        <li>Staged config editing with validation, diff, and apply flow</li>
        <li>DNS and DHCP editors plus lease management</li>
        <li>TFTP and PXE configuration with future asset lifecycle support</li>
        <li>Live logs and a Pi-hole-style operational dashboard</li>
      </ul>
    </article>

    <article class="panel">
      <h2>Foundation endpoints</h2>
      <ul>
        <li><code>/healthz</code> for liveness checks</li>
        <li><code>/api/v1/status</code> for runtime and shared-volume paths</li>
        <li><code>/api/v1/layout</code> for managed/manual/generated directories</li>
      </ul>
    </article>
  </section>
</main>

