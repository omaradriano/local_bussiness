import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { Image } from 'expo-image';
import { useState } from 'react';
import { Button, Pressable, Text, TextInput, View } from 'react-native';
import { api } from '../lib/api';
import { errorText, fieldErrors } from '../lib/errors';
import { pickPhotos, uploadStepPhoto } from '../lib/photos';

type Booking = components['schemas']['Booking'];
type Step = Booking['steps'][number];
const t = strings.bookings;

/**
 * One configured step of a booking (e.g. "Foto del INE"). How it's completed depends on its type;
 * unknown types are shown read-only (the API validates every submission with the type registry).
 */
export function StepCard({ businessId, bookingId, step, editable, onDone }: {
  businessId: string; bookingId: string; step: Step; editable: boolean; onDone: (b: Booking) => void;
}) {
  const [text, setText] = useState('');
  const [checked, setChecked] = useState<Record<string, boolean>>({});
  const [urls, setUrls] = useState<string[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const items = ((step.config as { items?: string[] }).items ?? []);
  const maxFiles = Number((step.config as { max_files?: number }).max_files ?? 6);
  const path = { businessId, bookingId, stepId: step.id };

  async function submit(body: { data?: Record<string, unknown>; files?: string[] }) {
    const { data, error, response } = await api.POST('/v1/b/{businessId}/bookings/{bookingId}/steps/{stepId}', { params: { path }, body });
    setBusy(false);
    if (data) return onDone(data);
    const f = fieldErrors(error);
    setMessage(f.data ?? f.files ?? errorText(error, response.status));
  }

  async function addPhotos() {
    setBusy(true);
    setMessage(null);
    const picked = await pickPhotos(maxFiles);
    if (picked === 'cancelled') return setBusy(false);
    if (typeof picked === 'string') { setBusy(false); return setMessage(picked); }
    const keys: string[] = [];
    for (const [i, photo] of picked.photos.entries()) {
      setMessage(strings.photos.progress(i + 1, picked.photos.length));
      const up = await uploadStepPhoto(businessId, bookingId, step.id, photo);
      if ('error' in up) { setBusy(false); return setMessage(up.error); }
      keys.push(up.key);
    }
    setMessage(null);
    await submit({ files: keys });
  }

  async function showFiles() {
    const out: string[] = [];
    for (const key of step.files) {
      const { data } = await api.GET('/v1/b/{businessId}/bookings/{bookingId}/files', {
        params: { path: { businessId, bookingId }, query: { bucket: step.bucket as 'private-docs' | 'rental-evidence', key } },
      });
      if (data) out.push(data.url);
    }
    setUrls(out);
  }

  const done = step.status === 'done';
  const target = step.to_status ? ` · antes de «${t.status[step.to_status as Booking['status']] ?? step.to_status}»` : '';
  return (
    <View style={{ borderWidth: 1, borderRadius: 8, padding: 10, gap: 6 }}>
      <Text style={{ fontWeight: '600' }}>{done ? '✓' : '○'} {step.label}{step.required ? '' : ' (opcional)'}</Text>
      <Text style={{ opacity: 0.7 }}>{done ? t.stepDone : t.stepPending}{target}</Text>

      {done && step.files.length > 0 && step.bucket !== '' && (
        urls ? (
          <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 6 }}>
            {urls.map((u) => <Image key={u} source={u} style={{ width: 96, height: 96, borderRadius: 6 }} contentFit="cover" accessibilityLabel={step.label} />)}
          </View>
        ) : (
          <Pressable accessibilityRole="button" onPress={showFiles}><Text style={{ textDecorationLine: 'underline' }}>{t.seeFiles} ({step.files.length})</Text></Pressable>
        )
      )}

      {!done && editable && (
        <>
          {(step.step_type === 'photo' || step.step_type === 'file_upload') && (
            <Button title={busy ? '…' : t.addStepPhotos} onPress={addPhotos} disabled={busy} />
          )}
          {(step.step_type === 'text' || step.step_type === 'number') && (
            <>
              <TextInput accessibilityLabel={step.label} placeholder={step.step_type === 'text' ? t.stepText : t.stepNumber} value={text} onChangeText={setText}
                keyboardType={step.step_type === 'number' ? 'decimal-pad' : 'default'} style={{ borderWidth: 1, borderRadius: 6, padding: 8 }} />
              <Button title={t.completeStep} disabled={busy} onPress={() => {
                setBusy(true);
                submit({ data: step.step_type === 'text' ? { text } : { value: Number(text) } });
              }} />
            </>
          )}
          {step.step_type === 'checklist' && (
            <>
              {items.map((it) => (
                <Pressable key={it} accessibilityRole="checkbox" accessibilityState={{ checked: !!checked[it] }}
                  onPress={() => setChecked({ ...checked, [it]: !checked[it] })} style={{ flexDirection: 'row', gap: 8 }}>
                  <Text>{checked[it] ? '☑' : '☐'}</Text><Text>{it}</Text>
                </Pressable>
              ))}
              <Button title={t.completeStep} disabled={busy} onPress={() => { setBusy(true); submit({ data: { checked } }); }} />
            </>
          )}
        </>
      )}
      {message && <Text accessibilityRole="alert">{message}</Text>}
    </View>
  );
}
