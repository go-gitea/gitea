// Basic tests
const assert = require("assert");

describe("Basic Tests", () => {
  it("should pass", () => {
    assert.strictEqual(1 + 1, 2);
  });

  it("should handle strings", () => {
    assert.strictEqual("hello" + " world", "hello world");
  });
});
