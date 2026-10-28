import { describe, expect, it } from 'vitest';
import { formatPhone, splitE164, toE164 } from './phone';

describe('phone', () => {
  it('builds E.164 only with the right number of digits', () => {
    expect(toE164('+52', '614 123 4567')).toBe('+526141234567');
    expect(toE164('+52', '614123456')).toBeNull();
    expect(toE164('+52', '61412345678')).toBeNull();
    expect(toE164('+1', '(915) 555-0100')).toBe('+19155550100');
  });

  it('splits back for editing', () => {
    expect(splitE164('+526141234567')).toEqual({ code: '+52', national: '6141234567' });
    expect(splitE164('+19155550100')).toEqual({ code: '+1', national: '9155550100' });
    expect(splitE164(null)).toEqual({ code: '+52', national: '' });
  });

  it('formats for display', () => expect(formatPhone('+526141234567')).toBe('+52 614 123 4567'));
});
