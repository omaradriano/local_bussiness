import { describe, expect, it } from 'vitest';
import { installMode } from './install';

const iphone = 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1';
const ipadOS = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15';
const android = 'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Mobile Safari/537.36';

describe('installMode', () => {
  it('hides the button when the app is already installed', () => {
    expect(installMode({ standalone: true, canPrompt: true, userAgent: android, touchPoints: 5 })).toBeNull();
    expect(installMode({ standalone: true, canPrompt: false, userAgent: iphone, touchPoints: 5 })).toBeNull();
  });
  it('uses the browser prompt when there is one (Android, Chrome, Edge)', () => {
    expect(installMode({ standalone: false, canPrompt: true, userAgent: android, touchPoints: 5 })).toBe('prompt');
  });
  it('shows the steps on iPhone and iPad, which have no prompt', () => {
    expect(installMode({ standalone: false, canPrompt: false, userAgent: iphone, touchPoints: 5 })).toBe('ios');
    expect(installMode({ standalone: false, canPrompt: false, userAgent: ipadOS, touchPoints: 5 })).toBe('ios');
  });
  it('hides the button where installing is not possible (Mac desktop, Firefox)', () => {
    expect(installMode({ standalone: false, canPrompt: false, userAgent: ipadOS, touchPoints: 0 })).toBeNull();
    expect(installMode({ standalone: false, canPrompt: false, userAgent: android, touchPoints: 5 })).toBeNull();
  });
});
