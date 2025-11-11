// @ts-check
import process from "node:process";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import tailwindcss from '@tailwindcss/vite';

/** @type {import("@astrojs/starlight/types").StarlightUserConfig['sidebar']} */
let cliSidebar = []
if (existsSync(join('.', 'cli-sidebar.json'))) {
    cliSidebar = [{
        label: 'CLI reference',
        items: JSON.parse(readFileSync(join('.', 'cli-sidebar.json'), 'utf-8'))
    }]
}

const version = readFileSync(join('..', 'VERSION'), 'utf-8').trim();

// https://astro.build/config
export default defineConfig({
    ...(process.env['ASTRO_SITE'] ? { site: process.env['ASTRO_SITE'] } : {}),
    ...(process.env['ASTRO_BASE'] ? { base: process.env['ASTRO_BASE'] } : {}),

    integrations: [
        starlight({
            title: 'Bumper',
            tagline: 'Changelog-driven development for your projects',
            social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/disintegrator/bumper' }],
            logo: {
                src: './src/assets/logo.svg',
            },
            customCss: [
                './src/styles/global.css',
                './src/styles/theme.css',
            ],
            components: {
                Hero: './src/components/hero.astro',
            },
            editLink: {
                baseUrl: "https://github.com/disintegrator/bumper/edit/main/site",
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
                ...cliSidebar,
                {
                    label: 'Changelog',
                    link: 'https://github.com/disintegrator/bumper/blob/main/CHANGELOG.md',
                },
            ],
        }),
    ],

    vite: {
        plugins: [tailwindcss()],
        define: {
            '__VERSION__': JSON.stringify(version),
        },
    },
});