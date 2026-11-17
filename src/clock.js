export const systemClock = {
  now() {
    return Date.now();
  },
  nowIso() {
    return new Date(this.now()).toISOString();
  },
};

export function createFixedClock(startIso) {
  let current = new Date(startIso).getTime();
  return {
    now() {
      return current;
    },
    nowIso() {
      return new Date(current).toISOString();
    },
    advance(ms) {
      current += ms;
      return new Date(current).toISOString();
    },
    set(iso) {
      current = new Date(iso).getTime();
      return new Date(current).toISOString();
    },
  };
}
