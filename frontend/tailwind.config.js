/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{svelte,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        primary: '#5B4C8C',
        secondary: '#7FA663',
        accent: '#F4A259',
      }
    },
  },
  plugins: [],
}