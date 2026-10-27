import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { Image } from 'expo-image';
import { useEffect, useState } from 'react';
import { Button, Pressable, Text, View } from 'react-native';
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

const box = { width: 96, height: 96, borderRadius: 6 } as const;

/** Photos of a catalog item: thumbnails, add and remove. */
export function ItemPhotos(props: Props) {
  const { businessId, editable } = props;
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
    if (typeof picked === 'string') { setBusy(false); return setMessage(picked); }
    const note = picked.dropped ? t.tooMany(MAX_PHOTOS) : null;
    if (props.itemId) {
      for (const [i, photo] of picked.photos.entries()) {
        setMessage(t.progress(i + 1, picked.photos.length));
        const err = await uploadPhoto(businessId, props.itemId, photo);
        if (err) { setBusy(false); props.reload(); return setMessage(err); }
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
    if (!response.ok) setMessage(errorText(error, response.status));
    props.reload?.();
  }

  const removeLink = (onPress: () => void) => editable && (
    <Pressable accessibilityRole="button" onPress={onPress}>
      <Text style={{ textDecorationLine: 'underline', fontSize: 12 }}>{t.remove}</Text>
    </Pressable>
  );

  return (
    <View style={{ gap: 8 }}>
      <Text style={{ fontWeight: '600' }}>{t.title}</Text>
      <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
        {images.map((img) => (
          <View key={img.id} style={{ width: 96, gap: 4 }}>
            {img.status === 'ready' && img.urls?.thumb ? (
              <Image source={img.urls.thumb} placeholder={img.blurhash ? { blurhash: img.blurhash } : undefined}
                accessibilityLabel={t.title} style={box} contentFit="cover" transition={150} />
            ) : (
              <View style={[box, { borderWidth: 1, justifyContent: 'center', alignItems: 'center' }]}>
                <Text style={{ fontSize: 12, textAlign: 'center' }}>{img.status === 'failed' ? t.failed : t.processing}</Text>
              </View>
            )}
            {removeLink(() => remove(img))}
          </View>
        ))}
        {pending.map((p, i) => (
          <View key={p.uri} style={{ width: 96, gap: 4 }}>
            <Image source={p.uri} accessibilityLabel={t.title} style={box} contentFit="cover" />
            {removeLink(() => props.onPendingChange?.(pending.filter((_, j) => j !== i)))}
          </View>
        ))}
      </View>
      {message && <Text accessibilityRole="alert">{message}</Text>}
      {editable && count < MAX_PHOTOS && <Button title={busy ? t.uploading : t.add} onPress={add} disabled={busy} />}
    </View>
  );
}
