import typography from '@tailwindcss/typography'

/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace'],
      },
      colors: {
        breed: {
          bianmu: '#4A90D9',
          xigou: '#E84393',
          jinmao: '#F39C12',
          demu: '#2C3E50',
          zangao: '#8E44AD',
          zhonghuatianyuanquan: '#27AE60',
        },
      },
      animation: {
        'pulse-border': 'pulse-border 2s infinite',
        'stream-glow': 'stream-glow 1.5s infinite ease-in-out',
      },
      keyframes: {
        'pulse-border': {
          '0%, 100%': { borderColor: 'rgba(225, 29, 72, 0.4)', boxShadow: '0 0 15px rgba(225, 29, 72, 0.2)' },
          '50%': { borderColor: 'rgba(225, 29, 72, 0.9)', boxShadow: '0 0 25px rgba(225, 29, 72, 0.5)' },
        },
        'stream-glow': {
          '0%, 100%': { opacity: '0.3' },
          '50%': { opacity: '0.8' },
        },
      },
    },
  },
  plugins: [typography],
};
