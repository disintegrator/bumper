// @ts-check
import process from "node:process";
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

import cliSidebar from './cli-sidebar.json' with { type: 'json' };

import tailwindcss from '@tailwindcss/vite';

// https://astro.build/config
export default defineConfig({
    ...(process.env['ASTRO_SITE'] ? { site: process.env['ASTRO_SITE'] } : {}),
    ...(process.env['ASTRO_BASE'] ? { base: process.env['ASTRO_BASE'] } : {}),

    integrations: [
        starlight({
            title: 'Bumper',
            tagline: 'Changelog-driven development for your projects',
            social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/disintegrator/bumper' }],
            customCss: [
                './src/styles/global.css',
                './src/styles/theme.css',
            ],
            components: {
                Hero: './src/components/hero.astro',
            },
            sidebar: [
                {
                    label: 'Introduction',
                    slug: 'introduction',
                },
                {
                    label: 'Installation',
                    slug: 'installation',
                },
                {
                    label: 'Guides',
                    autogenerate: { directory: 'guides' },
                },
                {
                    label: 'CI/CD Integrations',
                    autogenerate: { directory: 'ci-cd' },
                },
                {
                    label: 'Configuration',
                    autogenerate: { directory: 'configuration' },
                },
                {
                    label: 'Reference',
                    autogenerate: { directory: 'reference' },
                },
                {
                    label: 'CLI reference',
                    items: cliSidebar
                }
            ],
        }),
    ],

    vite: {
        plugins: [tailwindcss()],
    },
});