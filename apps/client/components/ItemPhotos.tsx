import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { ZoomableImage } from './ZoomableImage';
import { useEffect, useState } from 'react';
import { Ionicons } from '@expo/vector-icons';
import { Pressable, View } from 'react-native';
import { Button, Text, useToast, useUI } from '../ui';
import { api } from '../lib/api';
import { errorText } from '../lib/errors';
import { pickPhotos, uploadPhoto, type PickedPhoto } from '../lib/photos';

type CatalogImage = components['schemas']['CatalogImage'];
const t = strings.photos;
const MAX_PHOTOS = 8; // same limit as the API

type Props = { businessId: string; editable: boolean } & (
  // Existing item: photos upload right away.
  | { itemId: string; images: CatalogImage[]; reload: () => void; pending?: never; onPendingChange?: never }
  // New item: photos stay on the device until the item is saved (the form uploads them).
  | { itemId?: undefined; images?: never; reload?: never; pending: PickedPhoto[]; onPendingChange: (p: PickedPhoto[]) => void }
);

const box = { width: 88, height: 88, borderRadius: 10 } as const;

/** Photos of a catalog item: thumbnails, add and remove. */
export function ItemPhotos(props: Props) {
  const { businessId, editable } = props;
  const ui = useUI();
  const toast = useToast();
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const images = props.images ?? [];
  const pending = props.pending ?? [];
  const count = images.length + pending.length;
  const processing = images.some((i) => i.status === 'processing');
  const reload = props.reload;

  // Variants take a few seconds; poll every 3 s while any photo is processing (max ~40 s).
  useEffect(() => {
    if (!processing || !reload) return;
    let n = 0;
    const id = setInterval(() => { if (++n > 13) clearInterval(id); else reload(); }, 3000);
    return () => clearInterval(id);
  }, [processing, reload]);

  async function add() {
    setBusy(true);
    setMessage(null);
    const picked = await pickPhotos(MAX_PHOTOS - count);
    if (picked === 'cancelled') return setBusy(false);
    if (typeof picked === 'string') { setBusy(false); return toast(picked, 'error'); }
    const note = picked.dropped ? t.tooMany(MAX_PHOTOS) : null;
    if (props.itemId) {
      for (const [i, photo] of picked.photos.entries()) {
        setMessage(t.progress(i + 1, picked.photos.length));
        const err = await uploadPhoto(businessId, props.itemId, photo);
        if (err) { setBusy(false); setMessage(null); props.reload(); return toast(err, 'error'); }
      }
      props.reload();
    } else {
      props.onPendingChange?.([...pending, ...picked.photos]);
    }
    setMessage(note);
    setBusy(false);
  }

  async function remove(img: CatalogImage) {
    const { error, response } = await api.DELETE('/v1/b/{businessId}/catalog/{itemId}/images/{imageId}', {
      params: { path: { businessId, itemId: props.itemId!, imageId: img.id } },
    });
    if (!response.ok) toast(errorText(error, response.status), 'error');
    props.reload?.();
  }

  // A small ✕ on the corner of each photo (44 pt touch target through hitSlop).
  const removeLink = (onPress: () => void) => editable && (
    <Pressable accessibilityRole="button" accessibilityLabel={t.remove} onPress={onPress} hitSlop={12}
      style={{ position: 'absolute', top: -6, right: -6, width: 24, height: 24, borderRadius: 12, backgroundColor: ui.label, alignItems: 'center', justifyContent: 'center' }}>
      <Ionicons name="close" size={14} color={ui.bg} />
    </Pressable>
  );

  return (
    <View style={{ gap: 12 }}>
      <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
        {images.map((img) => (
          <View key={img.id} style={{ width: 88 }}>
            {img.status === 'ready' && img.urls?.thumb ? (
              <ZoomableImage source={img.urls.thumb} fullSource={img.urls.detail} placeholder={img.blurhash ? { blurhash: img.blurhash } : undefined}
                accessibilityLabel={t.title} style={box} contentFit="cover" transition={150} />
            ) : (
              <View style={[box, { backgroundColor: ui.fill, justifyContent: 'center', alignItems: 'center' }]}>
                <Text style={{ fontSize: 12, textAlign: 'center' }}>{img.status === 'failed' ? t.failed : t.processing}</Text>
              </View>
            )}
            {removeLink(() => remove(img))}
          </View>
        ))}
        {pending.map((p, i) => (
          <View key={p.uri} style={{ width: 88 }}>
            <ZoomableImage source={p.uri} accessibilityLabel={t.title} style={box} contentFit="cover" />
            {removeLink(() => props.onPendingChange?.(pending.filter((_, j) => j !== i)))}
          </View>
        ))}
      </View>
      {message && <Text variant="footnote" color="labelSecondary" accessibilityRole="alert">{message}</Text>}
      {editable && count < MAX_PHOTOS && <Button icon="images" busy={busy} busyLabel={t.uploading} onPress={add}>{t.add}</Button>}
    </View>
  );
}
