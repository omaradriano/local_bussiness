import { COUNTRY_CODES, onlyDigits, type CountryCode } from '@lb/shared';
import { Pressable, Text, TextInput, View } from 'react-native';

/** Country code (for WhatsApp) + national number, digits only. */
export function PhoneInput({ label, code, national, onChange, error }: {
  label: string; code: CountryCode; national: string; onChange: (code: CountryCode, national: string) => void; error?: string;
}) {
  const max = COUNTRY_CODES.find((c) => c.code === code)?.digits ?? 10;
  return (
    <View style={{ gap: 4 }}>
      <Text>{label}</Text>
      <View style={{ flexDirection: 'row', gap: 6 }}>
        {COUNTRY_CODES.map((c) => (
          <Pressable key={c.code} accessibilityRole="radio" accessibilityLabel={c.label} accessibilityState={{ checked: c.code === code }}
            onPress={() => onChange(c.code, national)}
            style={{ borderWidth: c.code === code ? 2 : 1, borderRadius: 6, paddingHorizontal: 10, justifyContent: 'center' }}>
            <Text>{c.code}</Text>
          </Pressable>
        ))}
        <TextInput accessibilityLabel={label} value={national} keyboardType="phone-pad" maxLength={max} placeholder={'0'.repeat(max)}
          onChangeText={(v) => onChange(code, onlyDigits(v))} style={{ flex: 1, borderWidth: 1, borderRadius: 6, padding: 10 }} />
      </View>
      {error && <Text accessibilityRole="alert">{error}</Text>}
    </View>
  );
}
