import type { Config } from '@react-router/dev/config'

export default {
  // SPA mode for static embedding in Go binary
  ssr: false,
  // Output to dist directory for Go embed
  buildDirectory: './dist',
} satisfies Config
