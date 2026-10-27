import type { components } from '@lb/api-client';
import { strings } from '@lb/shared';
import { Image } from 'expo-image';
import { useEffect, useState } from 'react';
import { Button, Pressable, Text, View } from 'react-native';
import { api } from '../lib/api';
import { errorText } from '../lib/errors';
import { pickAndUploadPhoto } from '../lib/photos';

type CatalogImage = components['schemas']['CatalogImage'];
const t = strings.photos;

/** Photos of a catalog item: thumbnails, add (pick + upload) and remove. Polls while some are processing. */
export function ItemPhotos({ businessId, itemId, images, reload, editable }: {
  businessId: string; itemId: string; images: CatalogImage[]; reload: () => void; editable: boolean;
}) {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const processing = images.some((i) => i.status === 'processing');

  // Variants take a few seconds; poll every 3 s while any photo is processing (max ~40 s).
  useEffect(() => {
    if (!processing) return;
    let n = 0;
    const id = setInterval(() => { if (++n > 13) clearInterval(id); else reload(); }, 3000);
    return () => clearInterval(id);
  }, [processing, reload]);

  async function add() {
    setBusy(true);
    setMessage(null);
    const err = await pickAndUploadPhoto(businessId, itemId);
    setBusy(false);
    if (err === 'cancelled') return;
    if (err) setMessage(err);
    reload();
  }

  async function remove(img: CatalogImage) {
    const { error, response } = await api.DELETE('/v1/b/{businessId}/catalog/{itemId}/images/{imageId}', {
      params: { path: { businessId, itemId, imageId: img.id } },
    });
    if (!response.ok) setMessage(errorText(error, response.status));
    reload();
  }

  return (
    <View style={{ gap: 8 }}>
      <Text style={{ fontWeight: '600' }}>{t.title}</Text>
      <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
        {images.map((img) => (
          <View key={img.id} style={{ width: 96, gap: 4 }}>
            {img.status === 'ready' && img.urls?.thumb ? (
              <Image
                source={img.urls.thumb}
                placeholder={img.blurhash ? { blurhash: img.blurhash } : undefined}
                accessibilityLabel={t.title}
                style={{ width: 96, height: 96, borderRadius: 6 }}
                contentFit="cover"
                transition={150}
              />
            ) : (
              <View style={{ width: 96, height: 96, borderRadius: 6, borderWidth: 1, justifyContent: 'center', alignItems: 'center' }}>
                <Text style={{ fontSize: 12, textAlign: 'center' }}>{img.status === 'failed' ? t.failed : t.processing}</Text>
              </View>
            )}
            {editable && (
              <Pressable accessibilityRole="button" onPress={() => remove(img)}>
                <Text style={{ textDecorationLine: 'underline', fontSize: 12 }}>{t.remove}</Text>
              </Pressable>
            )}
          </View>
        ))}
      </View>
      {message && <Text accessibilityRole="alert">{message}</Text>}
      {editable && <Button title={busy ? t.uploading : t.add} onPress={add} disabled={busy} />}
    </View>
  );
}
