import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { Ionicons } from '@expo/vector-icons';
import { useEffect, useState } from 'react';
import { Pressable, View } from 'react-native';
import { api } from '../lib/api';
import { errorText } from '../lib/errors';
import { pickPdfs, pickPhotos, uploadBookingDocument, type PickedPhoto } from '../lib/photos';
import { Button, Field, Text, radius, useToast, useUI } from '../ui';
import { FileThumb } from './FileThumb';

type Booking = components['schemas']['Booking'];
const t = strings.bookings.docs;

/** Files attached to a booking with the name the business gives them (photos or PDF), plus adding one. */
export function BookingDocuments({ businessId, booking, editable, onDone }: {
  businessId: string; booking: Booking; editable: boolean; onDone: (b: Booking) => void;
}) {
  const ui = useUI();
  const toast = useToast();
  const [name, setName] = useState('');
  const [nameError, setNameError] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);
  const [urls, setUrls] = useState<Record<string, string>>({});
  const path = { businessId, bookingId: booking.id };
  const docs = booking.documents ?? [];

  useEffect(() => {
    for (const d of docs) for (const key of d.files) {
      if (urls[key]) continue;
      api.GET('/v1/b/{businessId}/bookings/{bookingId}/files', { params: { path, query: { bucket: 'private-docs', key } } })
        .then(({ data }) => data && setUrls((u) => ({ ...u, [key]: data.url })));
    }
  }, [booking]); // eslint-disable-line react-hooks/exhaustive-deps

  async function add(kind: 'photo' | 'pdf') {
    if (!name.trim()) return setNameError(t.nameRequired);
    setNameError(undefined);
    setBusy(true);
    let files: PickedPhoto[] = [];
    if (kind === 'pdf') {
      const r = await pickPdfs(6);
      if (r !== 'cancelled') files = r;
    } else {
      const r = await pickPhotos(6);
      if (typeof r === 'string' && r !== 'cancelled') { setBusy(false); return toast(r, 'error'); }
      if (typeof r !== 'string') files = r.photos;
    }
    if (!files.length) return setBusy(false);
    const keys: string[] = [];
    for (const f of files) {
      const up = await uploadBookingDocument(businessId, booking.id, f);
      if ('error' in up) { setBusy(false); return toast(up.error, 'error'); }
      keys.push(up.key);
    }
    const { data, error, response } = await api.POST('/v1/b/{businessId}/bookings/{bookingId}/documents', { params: { path }, body: { name, files: keys } });
    setBusy(false);
    if (data) { setName(''); toast(t.saved); return onDone(data); }
    toast(errorText(error, response.status), 'error');
  }

  async function remove(docId: string) {
    setBusy(true);
    const { data, error, response } = await api.DELETE('/v1/b/{businessId}/bookings/{bookingId}/documents/{docId}', { params: { path: { ...path, docId } } });
    setBusy(false);
    if (data) { toast(t.removed); return onDone(data); }
    toast(errorText(error, response.status), 'error');
  }

  return (
    <View style={{ gap: 10 }}>
      {docs.map((d) => (
        <View key={d.id} style={{ borderRadius: radius.sm, padding: 14, backgroundColor: ui.fill, gap: 10 }}>
          <View style={{ flexDirection: 'row', alignItems: 'center', gap: 10 }}>
            <Ionicons name="document-attach" size={20} color={ui.accent} />
            <Text variant="headline" style={{ flex: 1 }}>{d.name}</Text>
            {editable && (
              <Pressable accessibilityRole="button" accessibilityLabel={`${t.remove} ${d.name}`} hitSlop={10} disabled={busy} onPress={() => remove(d.id)}>
                <Ionicons name="trash-outline" size={20} color={ui.danger} />
              </Pressable>
            )}
          </View>
          <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 10 }}>
            {d.files.map((key) => <FileThumb key={key} fileKey={key} url={urls[key]} label={d.name} />)}
          </View>
        </View>
      ))}
      {editable && (
        <View style={{ borderRadius: radius.sm, padding: 14, backgroundColor: ui.fill, gap: 4 }}>
          <Field label={t.name} placeholder={t.namePlaceholder} value={name} onChangeText={setName} error={nameError} style={{ backgroundColor: ui.surface }} />
          <View style={{ flexDirection: 'row', gap: 8 }}>
            <Button icon="camera" busy={busy} onPress={() => add('photo')} style={{ flex: 1 }}>{strings.files.addPhotos}</Button>
            <Button icon="document-text" disabled={busy} onPress={() => add('pdf')} style={{ flex: 1 }}>{strings.files.addPdf}</Button>
          </View>
        </View>
      )}
    </View>
  );
}
