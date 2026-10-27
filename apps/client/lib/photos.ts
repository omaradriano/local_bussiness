import { fitWithin, strings } from '@lb/shared';
import { ImageManipulator, SaveFormat } from 'expo-image-manipulator';
import * as ImagePicker from 'expo-image-picker';
import { api } from './api';
import { errorText, fieldErrors } from './errors';

/** A photo chosen and already shrunk on the device, not uploaded yet. */
export type PickedPhoto = { uri: string };

/**
 * Pick a photo and shrink it to 2048 px (JPEG; also applies the phone's EXIF rotation).
 * Returns the local photo, 'cancelled', or an error message.
 */
export async function pickPhoto(): Promise<PickedPhoto | 'cancelled' | string> {
  const perm = await ImagePicker.requestMediaLibraryPermissionsAsync();
  if (!perm.granted) return strings.photos.permission;

  const picked = await ImagePicker.launchImageLibraryAsync({ mediaTypes: 'images', quality: 1 });
  if (picked.canceled || !picked.assets[0]) return 'cancelled';
  const asset = picked.assets[0];

  const ctx = ImageManipulator.manipulate(asset.uri);
  const target = fitWithin(asset.width, asset.height);
  if (target) ctx.resize(target);
  const rendered = await ctx.renderAsync();
  const saved = await rendered.saveAsync({ format: SaveFormat.JPEG, compress: 0.85 });
  return { uri: saved.uri };
}

/** Upload a picked photo to an existing item: presigned PUT straight to S3, then confirm. Error message or null. */
export async function uploadPhoto(businessId: string, itemId: string, photo: PickedPhoto): Promise<string | null> {
  const blob = await (await fetch(photo.uri)).blob();
  const path = { businessId, itemId };
  const created = await api.POST('/v1/b/{businessId}/catalog/{itemId}/images/', {
    params: { path },
    body: { content_type: 'image/jpeg', size: blob.size },
  });
  if (!created.data) return fieldErrors(created.error).file ?? errorText(created.error, created.response.status);

  const put = await fetch(created.data.upload_url, { method: 'PUT', headers: created.data.upload_headers, body: blob });
  if (!put.ok) return `${strings.common.error} (S3 ${put.status})`;

  const done = await api.POST('/v1/b/{businessId}/catalog/{itemId}/images/{imageId}/complete', {
    params: { path: { ...path, imageId: created.data.image.id } },
  });
  return done.response.ok ? null : fieldErrors(done.error).file ?? errorText(done.error, done.response.status);
}

/** Upload several photos in order; returns the first error, if any. */
export async function uploadPhotos(businessId: string, itemId: string, photos: PickedPhoto[]): Promise<string | null> {
  for (const p of photos) {
    const err = await uploadPhoto(businessId, itemId, p);
    if (err) return err;
  }
  return null;
}
