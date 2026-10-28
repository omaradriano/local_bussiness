import { COUNTRY_CODES, onlyDigits, type CountryCode } from '@lb/shared';
import { TextInput, View } from 'react-native';
import { PressableScale, Text, radius, useUI } from '../ui';

/** Country code (for WhatsApp) + national number, digits only. */
export function PhoneInput({ label, code, national, onChange, error }: {
  label: string; code: CountryCode; national: string; onChange: (code: CountryCode, national: string) => void; error?: string;
}) {
  const ui = useUI();
  const max = COUNTRY_CODES.find((c) => c.code === code)?.digits ?? 10;
  return (
    <View style={{ gap: 6, marginBottom: 16 }}>
      <Text variant="footnote" color="labelSecondary" style={{ fontWeight: '600', paddingHorizontal: 4 }}>{label}</Text>
      <View style={{ flexDirection: 'row', gap: 6 }}>
        {COUNTRY_CODES.map((c) => (
          <PressableScale key={c.code} accessibilityRole="radio" accessibilityLabel={c.label} accessibilityState={{ checked: c.code === code }}
            onPress={() => onChange(c.code, national)}
            style={{ minHeight: 44, paddingHorizontal: 12, borderRadius: radius.sm, justifyContent: 'center', backgroundColor: c.code === code ? ui.accent : ui.fill }}>
            <Text style={{ color: c.code === code ? ui.onAccent : ui.label, fontWeight: '600' }}>{c.code}</Text>
          </PressableScale>
        ))}
        <TextInput accessibilityLabel={label} value={national} keyboardType="phone-pad" maxLength={max} placeholder={'0'.repeat(max)}
          placeholderTextColor={ui.labelSecondary} onChangeText={(v) => onChange(code, onlyDigits(v))}
          style={{ flex: 1, minWidth: 0, minHeight: 44, borderRadius: radius.sm, paddingHorizontal: 12, fontSize: 17, backgroundColor: ui.fill, color: ui.label,
            ...(error ? { borderWidth: 1, borderColor: ui.danger } : {}) }} />
      </View>
      {error && <Text variant="footnote" style={{ color: ui.danger, paddingHorizontal: 4 }}>{error}</Text>}
    </View>
  );
}
