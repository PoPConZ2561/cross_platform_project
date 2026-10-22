// Wails ไม่มี Node server จึงต้องเป็น SPA + static (nuxt generate -> .output/public)
export default defineNuxtConfig({
  ssr: false,
  devtools: { enabled: false },
  css: ['~/assets/main.css'],
  app: {
    head: {
      title: 'Photo Backup',
      link: [
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=IBM+Plex+Sans+Thai:wght@400;500;600&display=swap'
        }
      ]
    }
  },
  // ให้ Wails dev server ชี้มาที่ Nuxt dev server
  devServer: { port: 3000 }
})
