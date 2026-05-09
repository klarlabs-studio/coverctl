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
      description: 'Coverage feedback for AI coding agents — every language, every change. Domain-aware policy enforcement via MCP server (Claude Code, Cursor, Cline, Aider) and CLI.',
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
          label: 'Getting Started',
          items: [
            { label: 'Introduction', slug: '' },
            { label: 'Installation', slug: 'installation' },
            { label: 'Quick Start (Terminal)', slug: 'quick-start' },
            { label: 'Quick Start (AI Agent)', slug: 'quick-start-agent' },
            { label: 'MCP Server (AI Agents)', slug: 'mcp' },
          ],
        },
        {
          label: 'CLI Reference',
          items: [
            { label: 'Overview', slug: 'cli' },
            { label: 'check', slug: 'cli/check' },
            { label: 'run', slug: 'cli/run' },
            { label: 'watch', slug: 'cli/watch' },
            { label: 'init', slug: 'cli/init' },
            { label: 'report', slug: 'cli/report' },
            { label: 'Other Commands', slug: 'cli/other' },
          ],
        },
        {
          label: 'Configuration',
          items: [
            { label: 'Config File', slug: 'configuration' },
            { label: 'Domains', slug: 'configuration/domains' },
            { label: 'Policies', slug: 'configuration/policies' },
            { label: 'Advanced', slug: 'configuration/advanced' },
          ],
        },
        {
          label: 'Guides',
          items: [
            { label: 'CI Integration', slug: 'guides/ci-integration' },
            { label: 'Monorepo Support', slug: 'guides/monorepo' },
            { label: 'Build Flags', slug: 'guides/build-flags' },
          ],
        },
        {
          label: 'Architecture',
          items: [
            { label: 'Overview', slug: 'architecture' },
            { label: 'Contributing', slug: 'architecture/contributing' },
          ],
        },
      ],
    }),
  ],
});
