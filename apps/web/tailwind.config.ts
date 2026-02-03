import type { Config } from "tailwindcss";

const config = {
  darkMode: ["class"],
  content: [
    "./app/**/*.{ts,tsx,mdx}",
    "./components/**/*.{ts,tsx,mdx}",
    "./lib/**/*.{ts,tsx,mdx}",
    "./pages/**/*.{ts,tsx,mdx}",
    "./src/**/*.{ts,tsx,mdx}"
  ],
  theme: {
    extend: {
      transitionTimingFunction: {
        premium: "cubic-bezier(0.16, 1, 0.3, 1)"
      },
      colors: {
        bg: "hsl(var(--bg))",
        ink: "hsl(var(--ink))",
        muted: "hsl(var(--muted))",
        accent: "hsl(var(--accent))",
        accent2: "hsl(var(--accent-2))",
        "accent-warm": "hsl(var(--accent-warm))",
        line: "hsl(var(--line))",
        panel: "hsl(var(--panel))",
        "panel-2": "hsl(var(--panel-2))"
      },
      fontFamily: {
        display: ["var(--font-display)", "serif"],
        body: ["var(--font-body)", "sans-serif"]
      },
      keyframes: {
        gradientShift: {
          "0%": { backgroundPosition: "0% 50%" },
          "50%": { backgroundPosition: "100% 50%" },
          "100%": { backgroundPosition: "0% 50%" }
        },
        float: {
          "0%, 100%": { transform: "translateY(0px)" },
          "50%": { transform: "translateY(-8px)" }
        },
        fadeUp: {
          "0%": { opacity: "0", transform: "translateY(14px)" },
          "100%": { opacity: "1", transform: "translateY(0px)" }
        },
        lineGrow: {
          "0%": { opacity: "0", transform: "scaleY(0)" },
          "100%": { opacity: "1", transform: "scaleY(1)" }
        }
      },
      animation: {
        gradient: "gradientShift 20s ease infinite",
        float: "float 7s ease-in-out infinite",
        "fade-up": "fadeUp 0.6s ease-out both",
        "line-grow": "lineGrow 1.2s ease-out both"
      },
      boxShadow: {
        soft: "0 30px 80px -40px rgba(2, 6, 23, 0.75)",
        glow: "0 0 0 1px rgba(255, 255, 255, 0.08), 0 18px 40px -24px rgba(59, 130, 246, 0.6)"
      }
    }
  },
  plugins: [require("tailwindcss-animate")]
} satisfies Config;

export default config;
