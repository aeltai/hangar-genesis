<script setup lang="ts">
import type { HelmProduct } from '../types/genesis'

defineProps<{
  product: HelmProduct
}>()
</script>

<template>
  <div class="product-instructions">
    <p v-if="product.description" class="product-desc">{{ product.description }}</p>
    <p v-if="product.notes" class="product-notes">{{ product.notes }}</p>
    <h4 class="subtitle">Install the controller (Helm)</h4>
    <ol class="steps">
      <li>Add the Helm repository:</li>
    </ol>
    <pre class="code">helm repo add {{ product.helmRepoName }} {{ product.helmRepoUrl }}
helm repo update</pre>
    <p class="step-label">Install the controller:</p>
    <pre class="code">{{ product.helmInstallCmd }}</pre>
    <template v-if="product.cliReleasesUrl">
      <h4 class="subtitle">Install the CLI</h4>
      <p class="step-desc">
        Download the CLI from
        <a :href="product.cliReleasesUrl" target="_blank" rel="noopener noreferrer">{{ product.cliReleasesUrl }}</a>
        and place the binary in your PATH.
      </p>
    </template>
    <p v-if="product.docsUrl" class="docs-link">
      <a :href="product.docsUrl" target="_blank" rel="noopener noreferrer" class="docs-anchor">Documentation ↗</a>
    </p>
  </div>
</template>

<style scoped>
.product-instructions {
  margin-top: 0.75rem;
  padding: 1rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
  font-size: 0.9rem;
}
.product-desc {
  margin: 0 0 0.5rem 0;
  opacity: 0.95;
}
.product-notes {
  margin: 0 0 0.75rem 0;
  padding: 0.5rem;
  background: rgba(255, 255, 255, 0.05);
  border-radius: 4px;
  font-size: 0.85rem;
  opacity: 0.9;
}
.subtitle {
  margin: 0.75rem 0 0.35rem 0;
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--cyan);
}
.steps {
  margin: 0 0 0.25rem 0;
  padding-left: 1.25rem;
}
.step-label {
  margin: 0.5rem 0 0.25rem 0;
}
.step-desc {
  margin: 0.25rem 0 0;
  opacity: 0.9;
}
.code {
  margin: 0.35rem 0 0;
  padding: 0.6rem 0.75rem;
  background: rgba(0, 0, 0, 0.25);
  border-radius: 4px;
  font-family: ui-monospace, monospace;
  font-size: 0.85rem;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
.docs-link {
  margin: 0.75rem 0 0;
  font-size: 0.85rem;
}
.docs-anchor {
  color: var(--cyan);
  text-decoration: none;
  font-weight: 500;
}
.docs-anchor:hover {
  text-decoration: underline;
}
</style>
