import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "Octrafic Documentation",
  description: "AI-powered CLI tool for API testing and exploration",

  vite: {
    server: {
      allowedHosts: ['mbserver', 'localhost']
    }
  },

  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    logo: '/octrafic-logo.png',
    siteTitle: 'Octrafic',

    nav: [
      { text: 'Guide', link: '/getting-started/introduction' },
      { text: 'GitHub', link: 'https://github.com/Octrafic/octrafic-cli' }
    ],

    sidebar: [
      {
        text: 'Getting Started',
        items: [
          { text: 'Introduction', link: '/getting-started/introduction' },
          { text: 'Quick Start', link: '/getting-started/quick-start' }
        ]
      },
      {
        text: 'Guides',
        items: [
          { text: 'Project Management', link: '/guides/project-management' },
          { text: 'Authentication', link: '/guides/authentication' }
        ]
      }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/Octrafic/octrafic-cli' }
    ]
  }
})
