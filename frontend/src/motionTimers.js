// 动作相关定时器的统一管理：打断或新会话开始时一次性取消，并使仍在等待中的异步动作失效
export function createMotionTimers() {
  const timers = new Map();
  let epoch = 0;

  function cancel(name) {
    const id = timers.get(name);
    if (id !== undefined) {
      clearTimeout(id);
      timers.delete(name);
    }
  }

  return {
    // 当前代次；异步动作开始前记录，结束后与之比较，不一致说明期间发生过打断
    get epoch() {
      return epoch;
    },
    // 同名定时器会被覆盖；expectedEpoch 与当前不符时不再登记
    schedule(name, fn, ms, expectedEpoch = epoch) {
      if (expectedEpoch !== epoch) return false;
      cancel(name);
      timers.set(name, setTimeout(() => {
        timers.delete(name);
        fn();
      }, ms));
      return true;
    },
    cancel,
    // 取消全部定时器并推进代次
    cancelAll() {
      epoch += 1;
      Array.from(timers.keys()).forEach(cancel);
    },
    get pending() {
      return timers.size;
    }
  };
}
