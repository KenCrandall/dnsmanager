<script>
  import { onMount } from "svelte";

  let loading = true;
  let error = "";
  let status = null;
  let currentRevision = null;

  const trendBars = [42, 68, 56, 88, 73, 64, 92];

  onMount(async () => {
    try {
      const [statusResponse, revisionResponse] = await Promise.all([
        fetch("/api/v1/status"),
        fetch("/api/v1/config/revisions/current")
      ]);

      if (!statusResponse.ok) {
        throw new Error(`status request failed: ${statusResponse.status}`);
      }
      if (!revisionResponse.ok) {
        throw new Error(`current revision request failed: ${revisionResponse.status}`);
      }

      status = await statusResponse.json();
      currentRevision = await revisionResponse.json();
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
  <title>dnsmanager config lifecycle</title>
</svelte:head>

<main class="shell">
  <section class="hero">
    <div class="hero-copy">
      <p class="eyebrow">dnsmanager config lifecycle</p>
      <h1>Staged revision flow for a companion `dnsmasq` container.</h1>
      <p class="lede">
        This slice adds persisted draft revisions, staged rendering, validation,
        apply, and rollback primitives on top of the shared-volume Compose model
        so the next milestones can build actual DNS, DHCP, TFTP, and PXE editors.
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
      <h2>Lifecycle primitives</h2>
      <ul>
        <li>Draft revisions persisted in SQLite</li>
        <li>Staging-tree rendering with managed, manual, and generated areas</li>
        <li>`dnsmasq --test` validation when the binary is available</li>
        <li>Apply and rollback primitives for the active generated config</li>
      </ul>
    </article>

    <article class="panel">
      <h2>Current revision</h2>
      {#if currentRevision}
        <ul>
          <li>Revision #{currentRevision.id}: {currentRevision.summary}</li>
          <li>State: {currentRevision.state}</li>
          <li>Validation: {currentRevision.validationStatus}</li>
          <li>Created: {new Date(currentRevision.createdAt).toLocaleString()}</li>
        </ul>
        <p><code>{currentRevision.validationOutput}</code></p>
      {:else}
        <p>No revision state available yet.</p>
      {/if}
    </article>

    <article class="panel">
      <h2>Lifecycle endpoints</h2>
      <ul>
        <li><code>/healthz</code> for liveness checks</li>
        <li><code>/api/v1/status</code> for runtime and shared-volume paths</li>
        <li><code>/api/v1/config/revisions</code> for listing and creating drafts</li>
        <li><code>/api/v1/config/revisions/:id/validate</code> for validation/apply actions</li>
      </ul>
    </article>
  </section>
</main>
