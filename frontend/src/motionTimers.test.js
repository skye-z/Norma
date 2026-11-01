import test from 'node:test';
import assert from 'node:assert/strict';
import { createMotionTimers } from './motionTimers.js';

const wait = (ms) => new Promise((r) => setTimeout(r, ms));

test('打断后待机与表情复位定时器不再触发', async () => {
  const t = createMotionTimers();
  let fired = 0;
  t.schedule('idle', () => fired++, 20);
  t.schedule('expression', () => fired++, 20);
  assert.equal(t.pending, 2);
  t.cancelAll();
  assert.equal(t.pending, 0);
  await wait(60);
  assert.equal(fired, 0);
});

test('打断前发起的异步动作在返回后不能再登记定时器', async () => {
  const t = createMotionTimers();
  let fired = 0;
  const started = t.epoch;
  const motion = wait(10); // 模拟 await model.motion()
  t.cancelAll();
  await motion;
  assert.equal(t.schedule('idle', () => fired++, 10, started), false);
  await wait(40);
  assert.equal(fired, 0);
  assert.equal(t.pending, 0);
});

test('同名定时器被覆盖，只触发最后一次', async () => {
  const t = createMotionTimers();
  const calls = [];
  t.schedule('idle', () => calls.push(1), 30);
  t.schedule('idle', () => calls.push(2), 30);
  await wait(70);
  assert.deepEqual(calls, [2]);
});

test('打断之后的新会话可以正常登记定时器', async () => {
  const t = createMotionTimers();
  t.cancelAll();
  let fired = 0;
  assert.equal(t.schedule('idle', () => fired++, 10), true);
  await wait(40);
  assert.equal(fired, 1);
});
