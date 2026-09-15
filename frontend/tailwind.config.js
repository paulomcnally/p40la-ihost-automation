/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: [
    './index.html',
    './src/**/*.{js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      colors: {
        primary: 'rgb(var(--color-primary) / <alpha-value>)',
        'primary-hover': 'rgb(var(--color-primary-hover) / <alpha-value>)',
        danger: 'rgb(var(--color-danger) / <alpha-value>)',
        success: 'rgb(var(--color-success) / <alpha-value>)',
        warning: 'rgb(var(--color-warning) / <alpha-value>)',
        bg: 'rgb(var(--color-bg) / <alpha-value>)',
        card: 'rgb(var(--color-card) / <alpha-value>)',
        border: 'rgb(var(--color-border) / <alpha-value>)',
        'text-secondary': 'rgb(var(--color-text-secondary) / <alpha-value>)',
        text: 'rgb(var(--color-text) / <alpha-value>)',
      },
      borderRadius: {
        'ios': '12px',
        'ios-sm': '8px',
      },
      boxShadow: {
        'ios': '0 1px 3px rgba(0, 0, 0, 0.08)',
        'ios-lg': '0 4px 12px rgba(0, 0, 0, 0.12)',
      },
    },
  },
  plugins: [],
}