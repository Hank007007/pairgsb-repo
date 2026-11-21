/**
 * RRULE parsing, validation and bounded occurrence expansion.
 *
 * The accepted grammar is the one in docs/recur.md section 2. Anything the
 * service cannot evaluate predictably is rejected with a RuleError rather
 * than rendered or approximated.
 */

export const MAX_INSTANCES = 1000;

export class RuleError extends TypeError {}

const ACCEPTED = new Set([
  'FREQ',
  'INTERVAL',
  'COUNT',
  'UNTIL',
  'BYDAY',
  'BYMONTHDAY',
  'BYMONTH',
  'WKST',
]);

const FREQUENCIES = new Set(['DAILY', 'WEEKLY', 'MONTHLY', 'YEARLY']);
const DAY_CODES = ['MO', 'TU', 'WE', 'TH', 'FR', 'SA', 'SU'];
const DAY_INDEX = new Map(DAY_CODES.map((code, index) => [code, index]));
const FORMATTER_CACHE = new Map();

function fail(message) {
  throw new RuleError(message);
}

function positiveInteger(raw, name) {
  if (!/^\d+$/.test(String(raw))) fail(`${name} must be a positive integer`);
  const value = Number.parseInt(raw, 10);
  if (!Number.isSafeInteger(value) || value <= 0) fail(`${name} must be a positive integer`);
  return value;
}

function integerList(raw, name) {
  return String(raw)
    .split(',')
    .filter((part) => part.length > 0)
    .map((part) => {
      if (!/^-?\d+$/.test(part)) fail(`${name} must be a comma-separated list of integers`);
      return Number.parseInt(part, 10);
    });
}

export function isKnownZone(zone) {
  if (!zone || zone === 'UTC') return true;
  try {
    new Intl.DateTimeFormat('en-US', { timeZone: zone });
    return true;
  } catch {
    return false;
  }
}

function formatterFor(zone) {
  if (!FORMATTER_CACHE.has(zone)) {
    FORMATTER_CACHE.set(
      zone,
      new Intl.DateTimeFormat('en-US', {
        timeZone: zone,
        hour12: false,
        year: 'numeric',
        month: 'numeric',
        day: 'numeric',
        hour: 'numeric',
        minute: 'numeric',
        second: 'numeric',
      }),
    );
  }
  return FORMATTER_CACHE.get(zone);
}

function zoneParts(date, zone) {
  const parts = new Map();
  for (const part of formatterFor(zone).formatToParts(date)) {
    if (part.type !== 'literal') parts.set(part.type, part.value);
  }
  return {
    year: Number(parts.get('year')),
    month: Number(parts.get('month')),
    day: Number(parts.get('day')),
    hour: Number(parts.get('hour')) % 24,
    minute: Number(parts.get('minute')),
    second: Number(parts.get('second')),
  };
}

function offsetMillis(date, zone) {
  if (!zone || zone === 'UTC') return 0;
  const parts = zoneParts(date, zone);
  const wallUtc = Date.UTC(
    parts.year,
    parts.month - 1,
    parts.day,
    parts.hour,
    parts.minute,
    parts.second,
  );
  return wallUtc - Math.floor(date.getTime() / 1000) * 1000;
}

function wallToUtc(wall, zone) {
  const firstGuess = Date.UTC(
    wall.year,
    wall.month - 1,
    wall.day,
    wall.hour,
    wall.minute,
    wall.second,
  );
  if (!zone || zone === 'UTC') return new Date(firstGuess);
  const candidate = new Date(firstGuess - offsetMillis(new Date(firstGuess), zone));
  return new Date(firstGuess - offsetMillis(candidate, zone));
}

function dateToWall(date, zone) {
  if (!zone || zone === 'UTC') {
    return {
      year: date.getUTCFullYear(),
      month: date.getUTCMonth() + 1,
      day: date.getUTCDate(),
      hour: date.getUTCHours(),
      minute: date.getUTCMinutes(),
      second: date.getUTCSeconds(),
    };
  }
  return zoneParts(date, zone);
}

function addDays(wall, days) {
  const date = new Date(
    Date.UTC(wall.year, wall.month - 1, wall.day, wall.hour, wall.minute, wall.second) +
      days * 86400000,
  );
  return {
    year: date.getUTCFullYear(),
    month: date.getUTCMonth() + 1,
    day: date.getUTCDate(),
    hour: wall.hour,
    minute: wall.minute,
    second: wall.second,
  };
}

function daysInMonth(year, month) {
  return new Date(Date.UTC(year, month, 0)).getUTCDate();
}

function daysInYear(year) {
  return new Date(Date.UTC(year, 1, 29)).getUTCMonth() === 1 ? 366 : 365;
}

function addMonths(wall, months) {
  const monthIndex = wall.month - 1 + months;
  const year = wall.year + Math.floor(monthIndex / 12);
  const month = (((monthIndex % 12) + 12) % 12) + 1;
  if (wall.day > daysInMonth(year, month)) return null;
  return { ...wall, year, month };
}

function addYears(wall, years) {
  const year = wall.year + years;
  if (wall.month === 2 && wall.day === 29 && daysInMonth(year, 2) < 29) return null;
  return { ...wall, year };
}

function atWallTime(dayWall, timeWall) {
  return {
    year: dayWall.year,
    month: dayWall.month,
    day: dayWall.day,
    hour: timeWall.hour,
    minute: timeWall.minute,
    second: timeWall.second,
  };
}

function parseUntil(raw, zone) {
  if (/^\d{8}$/.test(raw)) {
    return {
      utc: new Date(
        Date.UTC(
          Number(raw.slice(0, 4)),
          Number(raw.slice(4, 6)) - 1,
          Number(raw.slice(6, 8)),
          23,
          59,
          59,
          999,
        ),
      ),
      dateOnly: true,
    };
  }
  if (/^\d{8}T\d{6}Z$/.test(raw)) {
    return {
      utc: new Date(
        Date.UTC(
          Number(raw.slice(0, 4)),
          Number(raw.slice(4, 6)) - 1,
          Number(raw.slice(6, 8)),
          Number(raw.slice(9, 11)),
          Number(raw.slice(11, 13)),
          Number(raw.slice(13, 15)),
        ),
      ),
      dateOnly: false,
    };
  }
  if (/^\d{8}T\d{6}$/.test(raw)) {
    if (!isKnownZone(zone)) fail('meeting time zone is not a recognised IANA zone');
    return {
      utc: wallToUtc(
        {
          year: Number(raw.slice(0, 4)),
          month: Number(raw.slice(4, 6)),
          day: Number(raw.slice(6, 8)),
          hour: Number(raw.slice(9, 11)),
          minute: Number(raw.slice(11, 13)),
          second: Number(raw.slice(13, 15)),
        },
        zone,
      ),
      dateOnly: false,
    };
  }
  fail('UNTIL must be YYYYMMDD or YYYYMMDDTHHMMSS[Z]');
}

function parseByDay(raw, freq) {
  const entries = [];
  for (const item of String(raw).split(',')) {
    const match = /^([+-]?\d+)?(MO|TU|WE|TH|FR|SA|SU)$/.exec(item);
    if (!match) fail(`invalid BYDAY value ${item}`);
    const ordinal = match[1] === undefined ? null : Number.parseInt(match[1], 10);
    if (ordinal === 0) fail(`invalid BYDAY value ${item}`);
    if (ordinal !== null && freq !== 'MONTHLY' && freq !== 'YEARLY') {
      fail(`ordinal BYDAY value ${item} is only valid for MONTHLY or YEARLY rules`);
    }
    entries.push({ ordinal, weekday: DAY_INDEX.get(match[2]) });
  }
  return entries;
}

/**
 * Parse an RRULE value into a validated descriptor.
 *
 * @param {string} rule
 * @param {string} [timeZone]
 * @returns {Record<string, unknown>|null}
 */
export function parseRRule(rule, timeZone) {
  if (rule === undefined || rule === null) return null;
  const text = String(rule).trim();
  if (text.length === 0) return null;

  const raw = {};
  for (const part of text.split(';')) {
    if (part.trim().length === 0) fail('RRULE contains an empty part');
    const eq = part.indexOf('=');
    if (eq <= 0) fail(`RRULE part ${part} is missing its value`);
    const name = part.slice(0, eq).trim().toUpperCase();
    const value = part.slice(eq + 1).trim();
    if (!ACCEPTED.has(name)) fail(`RRULE part ${name} is not supported`);
    if (Object.hasOwn(raw, name)) fail(`RRULE part ${name} is repeated`);
    if (value.length === 0) fail(`RRULE part ${name} has an empty value`);
    raw[name] = value;
  }

  if (!raw.FREQ) fail('RRULE requires FREQ');
  if (!FREQUENCIES.has(raw.FREQ)) fail(`RRULE FREQ ${raw.FREQ} is not supported`);
  if ('COUNT' in raw && 'UNTIL' in raw) {
    fail('RRULE COUNT and UNTIL are mutually exclusive');
  }

  const descriptor = {
    freq: raw.FREQ,
    interval: raw.INTERVAL === undefined ? 1 : positiveInteger(raw.INTERVAL, 'INTERVAL'),
    count: raw.COUNT === undefined ? null : positiveInteger(raw.COUNT, 'COUNT'),
    until: raw.UNTIL === undefined ? null : parseUntil(raw.UNTIL, timeZone),
    byDay: raw.BYDAY === undefined ? [] : parseByDay(raw.BYDAY, raw.FREQ),
    byMonthDay:
      raw.BYMONTHDAY === undefined
        ? []
        : integerList(raw.BYMONTHDAY, 'BYMONTHDAY').map((value) => {
            if (value < -31 || value === 0 || value > 31) {
              fail('BYMONTHDAY values must be in 1..31 or -31..-1');
            }
            return value;
          }),
    byMonth:
      raw.BYMONTH === undefined
        ? []
        : integerList(raw.BYMONTH, 'BYMONTH').map((value) => {
            if (value < 1 || value > 12) fail('BYMONTH values must be in 1..12');
            return value;
          }),
    wkst:
      raw.WKST === undefined
        ? null
        : DAY_INDEX.has(raw.WKST)
          ? DAY_INDEX.get(raw.WKST)
          : fail(`invalid WKST ${raw.WKST}`),
  };

  if (descriptor.count !== null && descriptor.count > MAX_INSTANCES) {
    fail(`RRULE COUNT ${descriptor.count} exceeds the expansion limit of ${MAX_INSTANCES}`);
  }
  return descriptor;
}

function stampZ(date) {
  return date
    .toISOString()
    .replace(/[-:]/g, '')
    .replace(/\.\d{3}Z$/, 'Z');
}

/**
 * Render a validated descriptor back into a canonical RRULE value.
 *
 * @param {Record<string, unknown>} descriptor
 * @returns {string}
 */
export function toRRuleString(descriptor) {
  const parts = [`FREQ=${descriptor.freq}`];
  if (descriptor.interval !== 1) parts.push(`INTERVAL=${descriptor.interval}`);
  if (descriptor.byDay.length > 0) {
    parts.push(
      'BYDAY=' +
        descriptor.byDay
          .map((entry) =>
            entry.ordinal === null
              ? DAY_CODES[entry.weekday]
              : `${entry.ordinal}${DAY_CODES[entry.weekday]}`,
          )
          .join(','),
    );
  }
  if (descriptor.byMonthDay.length > 0) {
    parts.push(`BYMONTHDAY=${descriptor.byMonthDay.join(',')}`);
  }
  if (descriptor.byMonth.length > 0) parts.push(`BYMONTH=${descriptor.byMonth.join(',')}`);
  if (descriptor.wkst !== null) parts.push(`WKST=${DAY_CODES[descriptor.wkst]}`);
  if (descriptor.count !== null) parts.push(`COUNT=${descriptor.count}`);
  if (descriptor.until) parts.push(`UNTIL=${stampZ(descriptor.until.utc)}`);
  return parts.join(';');
}

function weekdayOf(wall) {
  return new Date(Date.UTC(wall.year, wall.month - 1, wall.day)).getUTCDay();
}

function monthMatches(wall, descriptor) {
  return descriptor.byMonth.length === 0 || descriptor.byMonth.includes(wall.month);
}

function dayOfMonthMatches(wall, descriptor) {
  if (descriptor.byMonthDay.length === 0) return true;
  const lastDay = daysInMonth(wall.year, wall.month);
  return descriptor.byMonthDay.some((value) => {
    const day = value < 0 ? lastDay + 1 + value : value;
    return day === wall.day;
  });
}

function plainByDayMatches(wall, descriptor) {
  const plain = descriptor.byDay.filter((entry) => entry.ordinal === null);
  if (plain.length === 0) return true;
  return plain.some((entry) => entry.weekday === weekdayOf(wall));
}

function ordinalByDayMatches(wall, descriptor) {
  const ordinals = descriptor.byDay.filter((entry) => entry.ordinal !== null);
  if (ordinals.length === 0) return true;
  const lastDay = daysInMonth(wall.year, wall.month);
  return ordinals.some((entry) => {
    const matchingDays = [];
    for (let day = 1; day <= lastDay; day += 1) {
      if (weekdayOf({ year: wall.year, month: wall.month, day }) === entry.weekday) {
        matchingDays.push(day);
      }
    }
    const wanted =
      entry.ordinal > 0
        ? matchingDays[entry.ordinal - 1]
        : matchingDays[matchingDays.length + entry.ordinal];
    return wanted === wall.day;
  });
}

function candidateMatches(wall, descriptor) {
  if (!monthMatches(wall, descriptor)) return false;
  if (!plainByDayMatches(wall, descriptor)) return false;
  if (descriptor.byMonthDay.length > 0) {
    if (descriptor.byDay.some((entry) => entry.ordinal !== null)) {
      if (!ordinalByDayMatches(wall, descriptor)) return false;
    } else if (!dayOfMonthMatches(wall, descriptor)) {
      return false;
    }
  } else if (!ordinalByDayMatches(wall, descriptor)) {
    return false;
  }
  return true;
}

/**
 * Materialise the occurrences of a validated rule inside a half-open window.
 *
 * @param {Record<string, unknown>|string|null} rule
 * @param {{ start: Date, end: Date }} window
 * @param {string} [timeZone]
 * @returns {Date[]}
 */
export function expandOccurrences(rule, window, timeZone) {
  const descriptor = typeof rule === 'string' ? parseRRule(rule, timeZone) : rule;
  if (!descriptor || !window || !window.start || !window.end) return [];
  const zone = timeZone || 'UTC';
  const start = new Date(window.start);
  const end = new Date(window.end);
  const firstWall = dateToWall(start, zone);

  const until = descriptor.until ? descriptor.until.utc.getTime() : null;
  const count = descriptor.count;
  const occurrences = [];
  const seen = new Set();
  let seriesIndex = 0;
  let anchorIndex = 0;

  const emit = (wall) => {
    const instant = wallToUtc(wall, zone);
    const key = instant.getTime();
    if (seen.has(key)) return;
    if (instant < start || instant >= end) return;
    if (until !== null && key > until) return;
    if (count !== null && seriesIndex >= count) return;
    seen.add(key);
    seriesIndex += 1;
    occurrences.push(instant);
    if (occurrences.length > MAX_INSTANCES) {
      fail(`expansion exceeds the limit of ${MAX_INSTANCES} instances`);
    }
  };

  emit(firstWall);

  while (true) {
    if (count !== null && anchorIndex >= count) break;
    if (occurrences.length > MAX_INSTANCES) fail(`expansion exceeds the limit of ${MAX_INSTANCES} instances`);

    let candidates;
    if (descriptor.freq === 'DAILY' || descriptor.freq === 'WEEKLY') {
      const span = descriptor.freq === 'DAILY' ? 1 : 7;
      const first = addDays(firstWall, anchorIndex * descriptor.interval * span);
      const range = span;
      candidates = Array.from({ length: range }, (_, offset) => addDays(first, offset));
    } else if (descriptor.freq === 'MONTHLY') {
      const moved = addMonths(firstWall, anchorIndex * descriptor.interval);
      if (moved === null) {
        anchorIndex += 1;
        continue;
      }
      const lastDay = daysInMonth(moved.year, moved.month);
      candidates = Array.from({ length: lastDay }, (_, offset) =>
        atWallTime({ year: moved.year, month: moved.month, day: offset + 1 }, firstWall),
      );
    } else {
      const moved = addYears(firstWall, anchorIndex * descriptor.interval);
      if (moved === null) {
        anchorIndex += 1;
        continue;
      }
      const yearLength = daysInYear(moved.year);
      candidates = [];
      for (let dayOfYear = 1; dayOfYear <= yearLength; dayOfYear += 1) {
        const date = new Date(Date.UTC(moved.year, 0, dayOfYear));
        candidates.push(
          atWallTime(
            {
              year: date.getUTCFullYear(),
              month: date.getUTCMonth() + 1,
              day: date.getUTCDate(),
            },
            firstWall,
          ),
        );
      }
    }

    let advanced = false;
    for (const wall of candidates) {
      if (wall.year > 9999) fail('rule expands beyond year 9999');
      const instant = wallToUtc(wall, zone);
      if (count !== null && seriesIndex >= count) break;
      if (until !== null && instant.getTime() > until) break;
      if (instant >= end) break;
      if (candidateMatches(wall, descriptor)) {
        emit(wall);
        advanced = true;
      }
    }

    anchorIndex += 1;
    const nextAnchor =
      descriptor.freq === 'DAILY' || descriptor.freq === 'WEEKLY'
        ? addDays(
            firstWall,
            anchorIndex *
              descriptor.interval *
              (descriptor.freq === 'DAILY' ? 1 : 7),
          )
        : descriptor.freq === 'MONTHLY'
          ? addMonths(firstWall, anchorIndex * descriptor.interval)
          : addYears(firstWall, anchorIndex * descriptor.interval);
    if (nextAnchor === null) continue;
    if (wallToUtc(nextAnchor, zone) >= end) break;
    if (until !== null && wallToUtc(nextAnchor, zone).getTime() > until) break;
    if (count !== null && anchorIndex >= count) break;
    if (!advanced && anchorIndex > MAX_INSTANCES + 5) {
      fail(`expansion exceeds the limit of ${MAX_INSTANCES} instances`);
    }
  }

  return occurrences;
}
