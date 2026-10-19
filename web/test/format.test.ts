import { test } from "node:test";
import assert from "node:assert/strict";
import { changePercent, changePoints, changeSeconds, duration, niceScale, num, percent, share } from "../src/format.ts";

test("sayılar dile göre", () => {
  assert.equal(num(1204, "tr-TR"), "1.204");
  assert.equal(num(1204, "en-GB"), "1,204");
  assert.equal(percent(41.2, "tr-TR"), "%41");
  assert.equal(percent(41.2, "en-GB"), "41%");
});

test("oran uçları yuvarlanınca yanıltmaz", () => {
  assert.equal(share(0.49, "tr-TR"), "<%1");
  assert.equal(share(0.49, "en-GB"), "<1%");
  assert.equal(share(99.6, "en-GB"), ">99%");
  assert.equal(share(0, "en-GB"), "0%");
  assert.equal(share(100, "tr-TR"), "%100");
  assert.equal(share(18.4, "en-GB"), "18%");
});

test("süre", () => {
  assert.equal(duration(92, "tr"), "1 dk 32 sn");
  assert.equal(duration(45, "tr"), "45 sn");
  assert.equal(duration(7500, "tr"), "2 sa 5 dk");
  assert.equal(duration(92, "en"), "1m 32s");
});

test("değişim", () => {
  assert.deepEqual(changePercent(112, 100, "tr-TR"), { text: "+%12", up: true });
  assert.deepEqual(changePercent(88, 100, "tr-TR"), { text: "−%12", up: false });
  assert.equal(changePercent(5, 0, "tr-TR"), null); // önceki dönem boşsa yüzde yok
  assert.deepEqual(changePoints(41, 44, "puan"), { text: "−3 puan", up: false });
  assert.deepEqual(changeSeconds(92, 84, "tr"), { text: "+8 sn", up: true });
});

test("eksen ölçeği", () => {
  assert.deepEqual(niceScale(211), { max: 250, step: 50 });
  assert.deepEqual(niceScale(3), { max: 3, step: 1 });
  assert.deepEqual(niceScale(0), { max: 5, step: 1 });
  assert.deepEqual(niceScale(1840), { max: 2000, step: 500 });
});

import { serverMessage } from "../src/errors.ts";

test("sunucu hata mesajları", () => {
  assert.equal(serverMessage("mevcut şifre yanlış", "tr"), "Mevcut şifre yanlış");
  assert.equal(serverMessage("mevcut şifre yanlış", "en"), "Current password is wrong.");
  assert.equal(serverMessage("bilinmeyen bir şey", "en"), null);
  assert.equal(serverMessage("", "tr"), null);
});
