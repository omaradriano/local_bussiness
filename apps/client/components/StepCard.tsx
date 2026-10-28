import { Ionicons } from '@expo/vector-icons';
import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { ZoomableImage } from './ZoomableImage';
import { useEffect, useState } from 'react';
import { Pressable, TextInput, View } from 'react-native';
import { Button, Text, radius, useToast, useUI } from '../ui';
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
export function StepCard({ businessId, bookingId, step, editable, canChange, onDone }: {
  businessId: string; bookingId: string; step: Step; editable: boolean;
  /** While its moment hasn't passed a photo step can take more photos or drop one (the API checks it too). */
  canChange?: boolean; onDone: (b: Booking) => void;
}) {
  const ui = useUI();
  const toast = useToast();
  const [text, setText] = useState('');
  const [checked, setChecked] = useState<Record<string, boolean>>({});
  const [urls, setUrls] = useState<Record<string, string>>({}); // key -> signed URL
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
    setMessage(null);
    toast(f.data ?? f.files ?? errorText(error, response.status), 'error');
  }

  async function addPhotos() {
    setBusy(true);
    setMessage(null);
    const picked = await pickPhotos(Math.max(1, maxFiles - (step.status === 'done' ? step.files.length : 0)));
    if (picked === 'cancelled') return setBusy(false);
    if (typeof picked === 'string') { setBusy(false); return toast(picked, 'error'); }
    const keys: string[] = [];
    for (const [i, photo] of picked.photos.entries()) {
      setMessage(strings.photos.progress(i + 1, picked.photos.length));
      const up = await uploadStepPhoto(businessId, bookingId, step.id, photo);
      if ('error' in up) { setBusy(false); setMessage(null); return toast(up.error, 'error'); }
      keys.push(up.key);
    }
    setMessage(null);
    await submit({ files: keys });
  }

  async function showFiles() {
    for (const key of step.files) {
      if (urls[key]) continue;
      const { data } = await api.GET('/v1/b/{businessId}/bookings/{bookingId}/files', {
        params: { path: { businessId, bookingId }, query: { bucket: step.bucket as 'private-docs' | 'rental-evidence', key } },
      });
      if (data) setUrls((u) => ({ ...u, [key]: data.url }));
    }
  }

  async function removeFile(key: string) {
    setBusy(true);
    const { data, error, response } = await api.DELETE('/v1/b/{businessId}/bookings/{bookingId}/steps/{stepId}/files', {
      params: { path: { businessId, bookingId, stepId: step.id }, query: { key } },
    });
    setBusy(false);
    if (data) { toast(strings.photos.removed); return onDone(data); }
    toast(errorText(error, response.status), 'error');
  }

  const done = step.status === 'done';
  // Show the step's photos right away once it's done (short-lived signed URLs).
  useEffect(() => {
    if (done && step.files.length > 0 && step.bucket !== '') showFiles();
  }, [done, step.files.join()]); // eslint-disable-line react-hooks/exhaustive-deps

  const target = step.to_status ? ` · antes de «${t.status[step.to_status as Booking['status']] ?? step.to_status}»` : '';
  const fromProfile = step.customer_doc !== '' && step.on_file;
  return (
    <View style={{ borderRadius: radius.sm, padding: 14, backgroundColor: ui.fill, gap: 10 }}>
      <View style={{ flexDirection: 'row', gap: 10, alignItems: 'flex-start' }}>
        <Ionicons name={done || fromProfile ? 'checkmark-circle' : 'ellipse-outline'} size={22} color={done || fromProfile ? ui.success : ui.labelSecondary} />
        <View style={{ flex: 1, gap: 2 }}>
          <Text variant="headline">{step.label}{step.required ? '' : ' (opcional)'}</Text>
          <Text variant="footnote" color="labelSecondary">
            {fromProfile && !done ? strings.customers.fromProfile : done ? t.stepDone : t.stepPending}{target}
          </Text>
          {step.customer_doc !== '' && !step.on_file && !done && (
            <Text variant="footnote" color="labelSecondary">{strings.customers.missingDocs(strings.customers.docTypes[step.customer_doc] ?? step.label)}</Text>
          )}
        </View>
      </View>

      {done && step.files.length > 0 && (
        <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 10 }}>
          {step.files.map((key) => (
            <View key={key}>
              {urls[key]
                ? <ZoomableImage source={urls[key]} style={{ width: 80, height: 80, borderRadius: 10 }} contentFit="cover" accessibilityLabel={step.label} />
                : <View style={{ width: 80, height: 80, borderRadius: 10, backgroundColor: ui.surface }} />}
              {editable && canChange && !step.on_file && (
                <Pressable accessibilityRole="button" accessibilityLabel={strings.photos.remove} hitSlop={12} disabled={busy} onPress={() => removeFile(key)}
                  style={{ position: 'absolute', top: -6, right: -6, width: 24, height: 24, borderRadius: 12, backgroundColor: ui.label, alignItems: 'center', justifyContent: 'center' }}>
                  <Ionicons name="close" size={14} color={ui.bg} />
                </Pressable>
              )}
            </View>
          ))}
        </View>
      )}

      {done && editable && canChange && step.files.length < maxFiles && step.bucket !== '' && step.customer_doc === '' && (step.step_type === 'photo' || step.step_type === 'file_upload') && (
        <Button size="sm" icon="add" busy={busy} onPress={addPhotos}>{t.addMorePhotos}</Button>
      )}

      {!done && editable && !step.on_file && (
        <>
          {(step.step_type === 'photo' || step.step_type === 'file_upload') && (
            <Button icon="camera" busy={busy} onPress={addPhotos}>{t.addStepPhotos}</Button>
          )}
          {(step.step_type === 'text' || step.step_type === 'number') && (
            <>
              <TextInput accessibilityLabel={step.label} placeholder={step.step_type === 'text' ? t.stepText : t.stepNumber} value={text} onChangeText={setText}
                placeholderTextColor={ui.labelSecondary} keyboardType={step.step_type === 'number' ? 'decimal-pad' : 'default'}
                style={{ minHeight: 44, borderRadius: radius.sm, paddingHorizontal: 12, fontSize: 17, backgroundColor: ui.surface, color: ui.label }} />
              <Button disabled={busy} onPress={() => {
                setBusy(true);
                submit({ data: step.step_type === 'text' ? { text } : { value: Number(text) } });
              }}>{t.completeStep}</Button>
            </>
          )}
          {step.step_type === 'checklist' && (
            <>
              {items.map((it) => (
                <Pressable key={it} accessibilityRole="checkbox" accessibilityState={{ checked: !!checked[it] }}
                  onPress={() => setChecked({ ...checked, [it]: !checked[it] })} style={{ flexDirection: 'row', gap: 10, alignItems: 'center', minHeight: 36 }}>
                  <Ionicons name={checked[it] ? 'checkbox' : 'square-outline'} size={22} color={checked[it] ? ui.accent : ui.labelSecondary} />
                  <Text style={{ flex: 1 }}>{it}</Text>
                </Pressable>
              ))}
              <Button onPress={() => { setBusy(true); submit({ data: { checked } }); }} disabled={busy}>{t.completeStep}</Button>
            </>
          )}
        </>
      )}
      {message && <Text variant="footnote" color="labelSecondary">{message}</Text>}
    </View>
  );
}
