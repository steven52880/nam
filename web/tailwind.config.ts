import type { Config } from 'tailwindcss';
import animate from 'tailwindcss-animate';

const hsl = (name: string) => `hsl(var(--${name}) / <alpha-value>)`;

const config: Config = {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    fontSize: {
      xs: ['0.6875rem', '1rem'],
      sm: ['0.75rem', '1rem'],
      base: ['0.8125rem', '1.25rem'],
      lg: ['0.9375rem', '1.375rem'],
      xl: ['1.125rem', '1.625rem'],
      '2xl': ['1.375rem', '1.875rem'],
    },
    extend: {
      fontFamily: {
        sans: ['ui-sans-serif', 'system-ui', 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', 'sans-serif'],
        mono: ['ui-monospace', 'Cascadia Mono', 'Consolas', 'monospace'],
      },
      colors: {
        background: hsl('background'),
        foreground: hsl('foreground'),
        card: hsl('card'),
        popover: hsl('popover'),
        primary: { DEFAULT: hsl('primary'), foreground: hsl('primary-foreground') },
        muted: { DEFAULT: hsl('muted'), foreground: hsl('muted-foreground') },
        accent: { DEFAULT: hsl('accent'), foreground: hsl('accent-foreground') },
        destructive: { DEFAULT: hsl('destructive'), foreground: hsl('primary-foreground') },
        success: hsl('success'),
        warning: hsl('warning'),
        border: hsl('border'),
        input: hsl('input'),
        ring: hsl('ring'),
      },
      borderRadius: { DEFAULT: '0.25rem', md: '0.3125rem', lg: '0.375rem' },
    },
  },
  plugins: [animate],
};

export default config;
