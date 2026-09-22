export default {
  darkMode: ['class', '.dark'],
  content: [
    './components/**/*.{vue,js,ts}',
    './composables/**/*.{js,ts}',
    './pages/**/*.vue',
    './app.vue',
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Vazirmatn', 'IRANSans', 'Tahoma', 'Segoe UI', 'sans-serif'],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace'],
      },
    },
  },
  plugins: [],
}
