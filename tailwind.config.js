/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./components/*.templ"],
  theme: {
    extend: {
      colors: {
        discord_blurple: '#5865F2',
        twitch_purple: '#6441A5',
        xbox_green: '#107C10',
        minecraft_green: '#5D7C15',
      }
    },
  },
  plugins: [
    require('@tailwindcss/forms'),
    require('@tailwindcss/typography')
  ],
}
