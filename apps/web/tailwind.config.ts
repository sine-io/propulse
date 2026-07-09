import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: ["class"],
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        brand: {
          navy: "#0F2742",
          teal: "#17C3A5",
        },
      },
      fontFamily: {
        sans: ["Inter", "var(--font-geist-sans)", "Arial", "sans-serif"],
      },
    },
  },
  plugins: [require("tailwindcss-animate")],
};

export default config;
