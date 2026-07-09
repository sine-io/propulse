import { afterEach, describe, expect, it, vi } from "vitest";

import {
  ApiError,
  createCapacityCalculation,
  getActionWindow,
  getWatchlist,
  type HousingCapacityInput,
} from "./api-client";

const jsonResponse = (body: unknown, init?: ResponseInit) =>
  new Response(JSON.stringify(body), {
    headers: { "content-type": "application/json" },
    status: 200,
    ...init,
  });

describe("api-client", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("posts capacity calculation input as JSON", async () => {
    const input: HousingCapacityInput = {
      cashOnHand: 150,
      oldHomeValue: 320,
      oldLoanBalance: 80,
      monthlyIncome: 3.5,
      currentMonthlyMortgage: 0,
      acceptableMonthlyMortgage: 1.5,
      targetTotalPrice: 550,
      renovationBudget: 40,
      transactionCosts: 18,
      transitionRentCost: 5,
    };
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(
        jsonResponse({ id: "calculation_1", result: { pressureLevel: "safe", strategy: "可以同步推进" } }, { status: 201 }),
      );

    await expect(createCapacityCalculation(input)).resolves.toEqual({
      id: "calculation_1",
      result: { pressureLevel: "safe", strategy: "可以同步推进" },
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/capacity/calculations",
      expect.objectContaining({
        body: JSON.stringify(input),
        headers: { "content-type": "application/json" },
        method: "POST",
      }),
    );
  });

  it("reads the watchlist from the relative API route", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ items: [] }));

    await expect(getWatchlist()).resolves.toEqual({ items: [] });
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/watchlist", undefined);
  });

  it("passes an optional abort signal to requests", async () => {
    const signal = new AbortController().signal;
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(
        jsonResponse({
          action: "砍价",
          confidence: "高",
          summary: "预算和小区信号支持试探底价。",
          checklist: ["约看目标户型。"],
          risks: ["不要追高。"],
        }),
      );

    await getActionWindow(signal);

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/decision/action-window",
      { signal },
    );
  });

  it("turns API error JSON into ApiError code and message", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse(
        {
          error: {
            code: "capacity_required",
            message: "create a capacity calculation before requesting an action window",
          },
        },
        { status: 400 },
      ),
    );

    await expect(getActionWindow()).rejects.toMatchObject({
      code: "capacity_required",
      message: "create a capacity calculation before requesting an action window",
    });
    await expect(getActionWindow()).rejects.toBeInstanceOf(ApiError);
  });
});
