import { describe, expect, it } from 'vitest';
import { normalizeTime, shiftsToWeek, weekToShifts, type Shift } from './hours';

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
