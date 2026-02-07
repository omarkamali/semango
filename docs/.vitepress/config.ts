import { defineConfig } from 'vitepress'

export default defineConfig({
	title: '🥭 Semango',
	description: 'Hybrid search for your codebase and docs (BM25 + vectors).',
	lang: 'en-US',
	lastUpdated: true,
	cleanUrls: true,
	ignoreDeadLinks: true,
	srcExclude: [
		'SEMANGO_GUIDE.md',
		'LOCAL_EMBEDDER.md',
		'tabular.md',
		'getting-started.md',
		'api.md',
		'config-schema.md',
	],

	head: [
		['meta', { name: 'theme-color', content: '#3b82f6' }],
		['meta', { property: 'og:title', content: '🥭 Semango' }],
		['meta', { property: 'og:description', content: 'Hybrid search for your codebase and docs (BM25 + vectors).' }],
		['meta', { property: 'og:type', content: 'website' }],
		['meta', { property: 'og:url', content: 'https://semango.org/' }],
		['link', { rel: 'icon', href: '/favicon.svg' }],
	],

	themeConfig: {
		logo: '/mango.svg',
		nav: [
			{ text: 'Docs', link: '/guide/' },
			{ text: 'GitHub', link: 'https://github.com/omarkamali/semango' },
			{ text: 'Releases', link: 'https://github.com/omarkamali/semango/releases' },
		],
		search: {
			provider: 'local',
		},
		sidebar: [
			{
				text: 'Overview',
				items: [
					{ text: 'Guide', link: '/guide/' },
					{ text: 'Quickstart', link: '/guide/quickstart' },
					{ text: 'Install', link: '/guide/install' },
				],
			},
			{
				text: 'Core Concepts',
				items: [
					{ text: 'Configuration', link: '/guide/configuration' },
					{ text: 'Ingestion', link: '/guide/ingestion' },
					{ text: 'API', link: '/guide/api' },
					{ text: 'CLI', link: '/guide/cli' },
				],
			},
			{
				text: 'Embeddings',
				items: [
					{ text: 'Local (ONNX)', link: '/guide/embeddings-local' },
					{ text: 'Remote (OpenAI-compatible)', link: '/guide/embeddings-remote' },
				],
			},
			{
				text: 'Integrations',
				items: [
					{ text: 'MCP', link: '/guide/mcp' },
				],
			},
		],
		socialLinks: [
			{ icon: 'github', link: 'https://github.com/omarkamali/semango' },
		],
		footer: {
			message: 'Built by Omar Kamali (omarkamali.com) · Omneity Labs (omneitylabs.com) · MIT License',
			copyright: 'Copyright © Semango contributors',
		},
	},
})
