import { test } from "node:test";
import assert from "node:assert/strict";
import { apiQuery, parseState, stateSearch, withFilter, withoutFilter } from "../src/state.ts";

test("varsayılan durum", () => {
  assert.deepEqual(parseState(""), { period: "7d", from: "", to: "", metric: "visitors", filters: [] });
  assert.equal(stateSearch(parseState("")), "");
});

test("adres gidip geliyor", () => {
  const s = parseState("?period=custom&from=2026-10-01&to=2026-10-05&metric=pageviews&f=" + encodeURIComponent('[["country","TR"]]'));
  assert.equal(s.period, "custom");
  assert.deepEqual(s.filters, [["country", "TR"]]);
  assert.deepEqual(parseState(stateSearch(s)), s);
});

test("bozuk ya da tehlikeli adres güvenli değerlere düşer", () => {
  const s = parseState("?period=yil&metric=sifre&f=" + encodeURIComponent('[["password_hash","x"],["page","/a"],"bozuk"]'));
  assert.equal(s.period, "7d");
  assert.equal(s.metric, "visitors");
  assert.deepEqual(s.filters, [["page", "/a"]]);
  assert.deepEqual(parseState("?f=%7B").filters, []);
  assert.equal(parseState("?period=custom&from=dün").period, "7d"); // tarihsiz özel dönem
});

test("aynı boyutta tek filtre", () => {
  let s = parseState("");
  s = withFilter(s, ["country", "TR"]);
  s = withFilter(s, ["country", "DE"]);
  s = withFilter(s, ["page", "/"]);
  assert.deepEqual(s.filters, [["country", "DE"], ["page", "/"]]);
  assert.deepEqual(withoutFilter(s, "country").filters, [["page", "/"]]);
});

test("API sorgusu", () => {
  const s = withFilter(parseState("?period=day"), ["source", ""]);
  const q = new URLSearchParams(apiQuery(s, "Europe/Istanbul", { property: "page" }));
  assert.equal(q.get("period"), "day");
  assert.equal(q.get("tz"), "Europe/Istanbul");
  assert.equal(q.get("property"), "page");
  assert.equal(q.get("filters"), '[["source",""]]');
});
