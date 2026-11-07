import type { Config } from 'tailwindcss';
import animate from 'tailwindcss-animate';

const hsl = (name: string) => `hsl(var(--${name}) / <alpha-value>)`;

const config: Config = {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    fontSize: {
      xs: ['11px', '16px'],
      sm: ['12px', '16px'],
      base: ['13px', '20px'],
      lg: ['15px', '22px'],
      xl: ['18px', '26px'],
      '2xl': ['22px', '30px'],
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
      borderRadius: { DEFAULT: '4px', md: '5px', lg: '6px' },
    },
  },
  plugins: [animate],
};

export default config;
