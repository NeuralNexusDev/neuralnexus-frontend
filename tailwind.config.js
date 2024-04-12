/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./components/*.templ"],
  theme: {
    extend: {
      colors: {
        discord_blue: '#5865F2',
      }
    },
  },
  plugins: [
    require('@tailwindcss/forms'),
    require('@tailwindcss/typography')
  ],
}
