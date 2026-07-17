import { describe, expect, it } from "vitest";

import { buildManualRecords, createManualRow, type ManualRow } from "./manual-import";

function listingRow(overrides: Partial<ManualRow> = {}): ManualRow {
  return { ...createManualRow("listing"), layout: "三房", areaSqm: "138.6", price: "178", daysOnMarket: "45", status: "active", ...overrides };
}

function transactionRow(overrides: Partial<ManualRow> = {}): ManualRow {
  return { ...createManualRow("transaction"), layout: "三房", areaSqm: "120", price: "165", transactionDate: "2026-06-30", ...overrides };
}

describe("buildManualRecords", () => {
  it("converts a valid listing row into a listing record", () => {
    const { records, issues } = buildManualRecords([listingRow()]);
    expect(issues).toHaveLength(0);
    expect(records).toEqual([
      expect.objectContaining({ recordType: "listing", layout: "三房", areaSqm: 138.6, listingPrice: 178, daysOnMarket: 45, status: "active" }),
    ]);
  });

  it("converts a valid transaction row and keeps the optional listing ref", () => {
    const { records, issues } = buildManualRecords([transactionRow({ originalListingRef: "BK-9" })]);
    expect(issues).toHaveLength(0);
    expect(records[0]).toEqual(
      expect.objectContaining({ recordType: "transaction", transactionPrice: 165, transactionDate: "2026-06-30", originalListingRef: "BK-9" }),
    );
  });

  it("auto-generates a unique sourceRecordId when left blank", () => {
    const { records } = buildManualRecords([listingRow(), listingRow()]);
    expect(records[0].sourceRecordId).not.toEqual(records[1].sourceRecordId);
    expect(records[0].sourceRecordId).toMatch(/^manual-L-/);
  });

  it("flags out-of-range area and non-positive price", () => {
    const { records, issues } = buildManualRecords([listingRow({ areaSqm: "0", price: "-3" })]);
    expect(records).toHaveLength(0);
    expect(issues.map((issue) => issue.field)).toEqual(expect.arrayContaining(["areaSqm", "price"]));
  });

  it("rejects a transaction row with a malformed date", () => {
    const { records, issues } = buildManualRecords([transactionRow({ transactionDate: "2026/6/30" })]);
    expect(records).toHaveLength(0);
    expect(issues[0]).toMatchObject({ row: 1, field: "transactionDate" });
  });

  it("flags duplicate source record ids within the same record type", () => {
    const { issues } = buildManualRecords([listingRow({ sourceRecordId: "dup" }), listingRow({ sourceRecordId: "dup" })]);
    expect(issues).toContainEqual(expect.objectContaining({ row: 2, field: "sourceRecordId" }));
  });

  it("requires status and valid days on market for listings", () => {
    const { issues } = buildManualRecords([listingRow({ status: "", daysOnMarket: "-1" })]);
    expect(issues.map((issue) => issue.field)).toEqual(expect.arrayContaining(["daysOnMarket", "status"]));
  });
});
