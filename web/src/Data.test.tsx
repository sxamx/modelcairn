import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import Data from "./Data";
import type { Overview } from "./api/client";

test("shows only real operational totals and does not invent costs", () => {
  const overview = {
    resourceCounts: {},
    requests24h: { total: 10, success: 8, error: 2 },
    activeCooldowns: 1,
    recentRequests: [],
    generatedAt: "2026-09-21T00:00:00Z",
  } as Overview;
  render(<Data overview={overview} unavailable={false} />);
  expect(screen.getByText("80 %")).toBeInTheDocument();
  expect(screen.getByText("Costos estimados").closest("article")).toHaveTextContent("Sin datos");
  expect(screen.getByText("Latencia y tokens por segundo").closest("article")).toHaveTextContent("Sin datos agregados");
});
