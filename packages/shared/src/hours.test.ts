import { describe, expect, it } from 'vitest';
import { normalizeTime, shiftsToWeek, timeInHours, weekToShifts, type Shift } from './hours';

describe('shiftsToWeek / weekToShifts', () => {
  const shifts: Shift[] = [
    { weekday: 1, opens_at: '16:00', closes_at: '20:00' },
    { weekday: 1, opens_at: '09:00', closes_at: '14:00' },
    { weekday: 6, opens_at: '10:00', closes_at: '14:00' },
  ];

  it('builds 7 days with the earliest shift editable and the rest kept', () => {
    const week = shiftsToWeek(shifts);
    expect(week).toHaveLength(7);
    expect(week[0].open).toBe(false);
    expect(week[1]).toMatchObject({ open: true, opens_at: '09:00', closes_at: '14:00' });
    expect(week[1].extra).toEqual([{ weekday: 1, opens_at: '16:00', closes_at: '20:00' }]);
  });

  it('round-trips without losing split shifts', () => {
    const back = weekToShifts(shiftsToWeek(shifts));
    expect(back).toHaveLength(3);
    expect(back).toEqual(expect.arrayContaining(shifts));
  });

  it('closing a day drops all its shifts', () => {
    const week = shiftsToWeek(shifts);
    week[1].open = false;
    expect(weekToShifts(week).some((s) => s.weekday === 1)).toBe(false);
  });
});

describe('normalizeTime', () => {
  it.each([
    ['9:30', '09:30'],
    ['0930', '09:30'],
    ['23:59', '23:59'],
    [' 7:05 ', '07:05'],
  ])('%s -> %s', (input, want) => expect(normalizeTime(input)).toBe(want));

  it.each(['24:00', '9:60', 'nueve', '9'])('rejects %j', (input) => expect(normalizeTime(input)).toBeNull());
});

describe('timeInHours', () => {
  // Monday 2026-11-09, open 09:00–14:00 and 16:00–20:00; closed on Sunday.
  const hours: Shift[] = [{ weekday: 1, opens_at: '16:00', closes_at: '20:00' }, { weekday: 1, opens_at: '09:00', closes_at: '14:00' }];
  const at = (h: number, m = 0) => new Date(2026, 10, 9, h, m);

  it('keeps a time that fits', () => expect(timeInHours('2026-11-10', '10:30', [{ weekday: 2, opens_at: '09:00', closes_at: '20:00' }], at(21))).toBe('10:30'));
  it('moves a time after closing to the first opening of another day', () => {
    expect(timeInHours('2026-11-16', '22:00', hours, at(21))).toBe('09:00');
  });
  it('today: from the next full hour, skipping the break', () => {
    expect(timeInHours('2026-11-09', '22:00', hours, at(8, 10))).toBe('09:00');
    expect(timeInHours('2026-11-09', '22:00', hours, at(13, 30))).toBe('16:00');
    expect(timeInHours('2026-11-09', '10:00', hours, at(11, 5))).toBe('12:00');
  });
  it('no time left that day, or closed: empty', () => {
    expect(timeInHours('2026-11-09', '22:00', hours, at(19, 30))).toBe('');
    expect(timeInHours('2026-11-15', '12:00', hours, at(9))).toBe('');
  });
  it('without hours set only avoids the past', () => {
    expect(timeInHours('2026-11-09', '10:00', [], at(15, 2))).toBe('16:00');
    expect(timeInHours('2026-11-10', '10:00', [], at(15, 2))).toBe('10:00');
  });
});
