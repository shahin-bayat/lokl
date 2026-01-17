// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	site: 'https://shahin-bayat.github.io',
	base: '/lokl',
	integrations: [
		starlight({
			title: 'lokl',
			description: 'Local development environment orchestrator',
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/shahin-bayat/lokl' }],
			sidebar: [
				{
					label: 'Getting Started',
					items: [
						{ label: 'Introduction', slug: 'introduction' },
						{ label: 'Installation', slug: 'installation' },
						{ label: 'Quick Start', slug: 'quick-start' },
					],
				},
				{
					label: 'Configuration',
					items: [
						{ label: 'Config File', slug: 'config/file' },
						{ label: 'Services', slug: 'config/services' },
						{ label: 'Proxy & HTTPS', slug: 'config/proxy' },
					],
				},
				{
					label: 'CLI Reference',
					autogenerate: { directory: 'cli' },
				},
			],
		}),
	],
});
