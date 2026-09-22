import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import Data from "./Data";
import type { Overview } from "./api/client";

const overview = {
  resourceCounts: {}, requests24h: { total: 10, success: 8, error: 2 },
  activeCooldowns: 1, recentRequests: [], generatedAt: "2026-09-21T00:00:00Z",
} as Overview;
const response = (body: unknown) => Promise.resolve(new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }));
afterEach(() => vi.restoreAllMocks());

test("shows retained real tokens and estimates cost only when a model tariff exists", async () => {
  vi.spyOn(globalThis, "fetch").mockImplementation(input => String(input).endsWith("/metrics") ? response({
    requests: 2, inputTokens: 1000, outputTokens: 500, usageKnownRequests: 1,
    daily: [{ date: "2026-09-21", requests: 2, success: 1, inputTokens: 1000, outputTokens: 500 }],
    models: [{ name: "google-model", requests: 1, inputTokens: 1000, outputTokens: 500, avgLatencyMillis: 1200, outputTokensPerSecond: 25 }], generatedAt: "2026-09-21T12:00:00Z",
  }) : response({ items: [{ kind: "Model", metadata: { name: "google-model" }, spec: { pricing: { currency: "USD", inputPerMillion: 1, outputPerMillion: 2 } } }], nextCursor: null }));
  render(<Data overview={overview} unavailable={false}/>);
  await waitFor(() => expect(screen.getByText("Tokens registrados").closest("article")).toHaveTextContent("1500"));
  expect(screen.getByText("Disponibilidad observada").closest("article")).toHaveTextContent("80 %");
  expect(screen.getByText("Costo estimado").closest("article")).not.toHaveTextContent("—");
});

test("demo is explicitly marked and does not write or replace real metrics", async () => {
  const fetcher = vi.spyOn(globalThis, "fetch").mockImplementation(input => String(input).endsWith("/metrics") ? response({ requests: 0, inputTokens: 0, outputTokens: 0, usageKnownRequests: 0, daily: [], models: [], generatedAt: "2026-09-21T12:00:00Z" }) : response({ items: [], nextCursor: null }));
  render(<Data overview={overview} unavailable={false}/>);
  await screen.findByText("0 de 0 solicitudes tienen tokens de entrada y salida.");
  fireEvent.click(screen.getByRole("button", { name: "Ver datos de ejemplo" }));
  expect(screen.getByText("Vista de demostración")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Volver a datos reales" }));
  expect(screen.getByText("0 de 0 solicitudes tienen tokens de entrada y salida.")).toBeInTheDocument();
  expect(fetcher.mock.calls.every(([, init]) => !init || !init.method || init.method === "GET")).toBe(true);
});
