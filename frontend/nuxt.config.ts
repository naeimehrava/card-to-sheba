export default defineNuxtConfig({
  compatibilityDate: '2024-11-01',
  devtools: { enabled: false },
  modules: ['@nuxtjs/tailwindcss'],
  tailwindcss: {
    configPath: 'tailwind.config.js',
  },
  hooks: {
    'tailwindcss:config'(config) {
      config.darkMode = 'class'
    },
  },
  runtimeConfig: {
    public: {
      apiBaseUrl: process.env.NUXT_PUBLIC_API_BASE_URL || 'http://localhost:8080',
    },
  },
  app: {
    head: {
      htmlAttrs: { lang: 'fa', dir: 'rtl' },
      title: 'تبدیل شماره کارت به شبا',
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        { name: 'theme-color', content: '#f8fafc' },
        { name: 'color-scheme', content: 'light dark' },
      ],
      script: [
        {
          innerHTML:
            "(function(){try{var t=localStorage.getItem('theme');if(t==='dark'){document.documentElement.classList.add('dark');document.documentElement.style.colorScheme='dark'}else{document.documentElement.classList.remove('dark');document.documentElement.style.colorScheme='light'}}catch(e){}})()",
          tagPosition: 'head',
        },
      ],
    },
  },
})
