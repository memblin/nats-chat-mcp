import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["test/**/*.test.ts"],
    // Integration tests pull and boot a real NATS container — give them room.
    testTimeout: 60_000,
    hookTimeout: 120_000,
    // One broker per file; run files serially so containers don't contend.
    fileParallelism: false,
    coverage: {
      provider: "v8",
      reporter: ["text", "lcov"],
      reportsDirectory: "coverage",
      include: ["src/**/*.ts"],
    },
  },
});
