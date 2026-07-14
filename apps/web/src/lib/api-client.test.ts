import { afterEach, describe, expect, it, vi } from "vitest";

import { clearAccessToken, getAccessToken, setAccessToken } from "./access-token";

import {
  ApiError,
  getCSVImportTemplate,
  importCSVCollectionRun,
  importJSONCollectionRun,
  listDataSources,
  createCapacityCalculation,
  getActionWindow,
	getNeighborhood,
	getNeighborhoodMetrics,
	getMetricHistory,
  getWatchlist,
  verifyAccessToken,
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
    clearAccessToken();
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

	it("queries metric history with an encoded inclusive window", async () => {
		const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
			jsonResponse({
				status: "empty",
				neighborhoodId: "neighborhood/1",
				algorithmVersion: "market-metrics/test.1",
				window: { from: "2026-05-19T00:00:00Z", to: "2026-07-14T00:00:00Z" },
				items: [],
			}),
		);

		await getMetricHistory("neighborhood/1", {
			from: "2026-05-19T00:00:00Z",
			to: "2026-07-14T00:00:00Z",
		});

		expect(fetchMock).toHaveBeenCalledWith(
			"/api/v1/neighborhoods/neighborhood%2F1/metrics/history?from=2026-05-19T00%3A00%3A00Z&to=2026-07-14T00%3A00%3A00Z",
			undefined,
		);
	});

	it("reads neighborhood identity and latest metrics with encoded IDs", async () => {
		const fetchMock = vi.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(jsonResponse({ id: "neighborhood/1", name: "接口花园", area: "南城", targetLayout: "两房" }))
			.mockResolvedValueOnce(jsonResponse({ id: "metric-1", neighborhoodId: "neighborhood/1" }));
		const signal = new AbortController().signal;

		await getNeighborhood("neighborhood/1", signal);
		await getNeighborhoodMetrics("neighborhood/1", signal);

		expect(fetchMock).toHaveBeenNthCalledWith(
			1,
			"/api/v1/neighborhoods/neighborhood%2F1",
			{ signal },
		);
		expect(fetchMock).toHaveBeenNthCalledWith(
			2,
			"/api/v1/neighborhoods/neighborhood%2F1/metrics",
			{ signal },
		);
	});

  it("validates and attaches bearer access tokens", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ status: "unlocked" }));

    await expect(verifyAccessToken("secret-token")).resolves.toEqual({
      status: "unlocked",
    });
    const init = fetchMock.mock.calls[0]?.[1];
    expect(new Headers(init?.headers).get("Authorization")).toBe(
      "Bearer secret-token",
    );
  });

  it("uses the session token and clears it after a 401", async () => {
    setAccessToken("expired-token");
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse(
        {
          error: {
            code: "access_required",
            message: "valid bearer access token is required",
          },
        },
        { status: 401 },
      ),
    );

    await expect(getWatchlist()).rejects.toMatchObject({ status: 401 });
    const init = fetchMock.mock.calls[0]?.[1];
    expect(new Headers(init?.headers).get("Authorization")).toBe(
      "Bearer expired-token",
    );
    expect(getAccessToken()).toBeUndefined();
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

  it("reads protected data sources with the session token", async () => {
    setAccessToken("data-token");
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse({ items: [{ id: "source-1", name: "Source" }] }),
    );

    await expect(listDataSources()).resolves.toEqual([
      { id: "source-1", name: "Source" },
    ]);
    const init = fetchMock.mock.calls[0]?.[1];
    expect(new Headers(init?.headers).get("Authorization")).toBe(
      "Bearer data-token",
    );
  });

  it("submits JSON records with collection metadata", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse({ collectionRunId: "run-1" }, { status: 201 }),
    );
    const input = {
      dataSourceId: "source-1",
      neighborhoodId: "neighborhood-1",
      sourceRef: "weekly-1",
      collectedAt: "2026-07-14T06:00:00.000Z",
      coverage: "full" as const,
      records: [
        {
          recordType: "listing" as const,
          sourceRecordId: "listing-1",
          layout: "three-bedroom",
          areaSqm: 89,
          listingPrice: 520,
          daysOnMarket: 12,
          status: "active" as const,
        },
      ],
    };

    await importJSONCollectionRun(input);

    expect(fetchMock).toHaveBeenCalledWith(
      "/admin/api/imports/json",
      expect.objectContaining({
        body: JSON.stringify(input),
        method: "POST",
      }),
    );
  });

  it("submits CSV imports as multipart without overriding its content type", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse({ collectionRunId: "run-1" }, { status: 201 }),
    );
    const file = new File(["recordType\n"], "records.csv", {
      type: "text/csv",
    });

    await importCSVCollectionRun(
      {
        dataSourceId: "source-1",
        neighborhoodId: "neighborhood-1",
        sourceRef: "weekly-1",
        collectedAt: "2026-07-14T06:00:00.000Z",
        coverage: "partial",
      },
      file,
    );

    const init = fetchMock.mock.calls[0]?.[1];
    expect(init?.body).toBeInstanceOf(FormData);
    expect((init?.body as FormData).get("file")).toBe(file);
    expect(new Headers(init?.headers).has("content-type")).toBe(false);
  });

  it("preserves import validation details and counts on ApiError", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse(
        {
          error: {
            code: "validation_failed",
            message: "one or more fields are invalid",
            details: [
              {
                row: 3,
                field: "listingPrice",
                code: "required",
                message: "listingPrice is required",
              },
            ],
          },
          acceptedRecordCount: 0,
          rejectedRecordCount: 2,
        },
        { status: 422 },
      ),
    );

    await expect(
      importJSONCollectionRun({
        dataSourceId: "source-1",
        neighborhoodId: "neighborhood-1",
        sourceRef: "weekly-1",
        collectedAt: "2026-07-14T06:00:00.000Z",
        coverage: "full",
        records: [],
      }),
    ).rejects.toMatchObject({
      code: "validation_failed",
      rejectedRecordCount: 2,
      details: [expect.objectContaining({ row: 3, field: "listingPrice" })],
    });
  });

  it("downloads the protected CSV template as a blob", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("recordType,sourceRecordId\r\n", {
        headers: { "content-type": "text/csv" },
      }),
    );

    const blob = await getCSVImportTemplate();

    expect(await blob.text()).toContain("recordType,sourceRecordId");
  });
});
