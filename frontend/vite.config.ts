// SPDX-License-Identifier: AGPL-3.0-or-later
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  // css: true so a test can read a stylesheet's text with `?raw` and assert on
  // it (src/styles/pro-material.test.ts); vitest otherwise stubs .css imports
  // to an empty module, even the raw ones.
  test: { include: ['src/**/*.test.{ts,tsx}'], css: true },
})
