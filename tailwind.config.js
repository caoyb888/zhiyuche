export default {
  content: ["./index.html", "./src/**/*.{js,jsx}"],
  theme: {
    extend: {
      colors: {
        brand: { 50:'#eef6ff', 100:'#dbeafe', 200:'#bfdbfe', 400:'#60a5fa', 600:'#1d6fd8', 700:'#1e4fa8', 800:'#1e3a6e', 900:'#0f1f3d' },
        ev:    { 400:'#34d399', 500:'#10b981', 600:'#059669' },
      },
      fontFamily: {
        sans: ['"DM Sans"', 'sans-serif'],
        mono: ['"DM Mono"', 'monospace'],
      }
    }
  },
  plugins: []
}
