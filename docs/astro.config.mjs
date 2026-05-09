// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
  site: 'https://felixgeelhaar.github.io',
  base: '/coverctl',
  vite: {
    build: {
      rollupOptions: {
        onwarn(warning, warn) {
          if (warning.code === 'UNUSED_EXTERNAL_IMPORT') {
            const message = String(warning.message || '');
            const importer = String(warning.importer || '');
            const source = String(warning.source || '');
            if (
              message.includes('@astrojs/internal-helpers/remote') ||
              importer.includes('astro/dist/assets/utils/remotePattern.js') ||
              source.includes('@astrojs/internal-helpers/remote')
            ) {
              return;
            }
          }
          warn(warning);
        },
      },
    },
  },
  integrations: [
    starlight({
      title: 'coverctl',
      description: 'Agent-loop coverage governance — coverage your AI coding agent calls before commit, not a dashboard you read after CI. MCP-native, polyglot, local-first.',
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/felixgeelhaar/coverctl',
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/felixgeelhaar/coverctl/edit/main/docs/',
      },
      customCss: ['./src/styles/custom.css'],
      head: [
        {
          tag: 'meta',
          attrs: {
            property: 'og:image',
            content: 'https://felixgeelhaar.github.io/coverctl/og-image.png',
          },
        },
        {
          tag: 'link',
          attrs: {
            rel: 'preconnect',
            href: 'https://fonts.googleapis.com',
          },
        },
        {
          tag: 'link',
          attrs: {
            rel: 'preconnect',
            href: 'https://fonts.gstatic.com',
            crossorigin: 'anonymous',
          },
        },
      ],
      expressiveCode: {
        themes: ['github-dark', 'github-light'],
        styleOverrides: {
          borderRadius: '10px',
          codeFontFamily: "'JetBrains Mono', 'Fira Code', monospace",
        },
      },
      sidebar: [
        {
          label: 'Start here',
          items: [
            { label: 'Introduction', slug: '' },
            { label: 'Quick start (AI agent)', slug: 'quick-start-agent' },
            { label: 'Quick start (terminal)', slug: 'quick-start' },
            { label: 'Install', slug: 'installation' },
          ],
        },
        {
          label: 'Use coverctl with your agent',
          items: [
            { label: 'The agent loop, end to end', slug: 'agent-loop-tutorial' },
            { label: 'MCP server reference', slug: 'mcp' },
          ],
        },
        {
          label: 'For platform & devex teams',
          items: [
            { label: 'Overview', slug: 'for-platform-teams' },
            { label: 'Threat model', slug: 'security/threat-model' },
            { label: 'Rejection schema', slug: 'security/rejection-schema' },
          ],
        },
        {
          label: 'Configure and operate',
          items: [
            { label: 'Policy basics', slug: 'configuration' },
            { label: 'Domains', slug: 'configuration/domains' },
            { label: 'Policies', slug: 'configuration/policies' },
            { label: 'Advanced', slug: 'configuration/advanced' },
            { label: 'CI integration', slug: 'guides/ci-integration' },
            { label: 'Monorepos', slug: 'guides/monorepo' },
          ],
        },
        {
          label: 'Compare',
          items: [
            { label: 'coverctl vs Codecov', slug: 'compare/coverctl-vs-codecov' },
            { label: 'coverctl vs Coveralls', slug: 'compare/coverctl-vs-coveralls' },
            { label: 'coverctl vs native commands', slug: 'compare/coverctl-vs-native' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'CLI overview', slug: 'cli' },
            { label: 'check', slug: 'cli/check' },
            { label: 'run', slug: 'cli/run' },
            { label: 'watch', slug: 'cli/watch' },
            { label: 'init', slug: 'cli/init' },
            { label: 'report', slug: 'cli/report' },
            { label: 'Other commands', slug: 'cli/other' },
            { label: 'Build flags', slug: 'guides/build-flags' },
            { label: 'Pricing & roadmap', slug: 'pricing' },
          ],
        },
      ],
    }),
  ],
});
