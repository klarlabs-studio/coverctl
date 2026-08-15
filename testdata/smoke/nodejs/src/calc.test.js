import { test } from "node:test";
import assert from "node:assert/strict";
import { add, mul } from "./calc.js";

test("add", () => {
  assert.equal(add(2, 3), 5);
});

test("mul", () => {
  assert.equal(mul(2, 4), 8);
});
